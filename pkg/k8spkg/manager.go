package k8spkg

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mgoltzsche/k8spkg/pkg/client"
	"github.com/mgoltzsche/k8spkg/pkg/resource"
	"github.com/mgoltzsche/k8spkg/pkg/status"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	// See https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/
	PKG_NAME_LABEL = "app.kubernetes.io/part-of"
	PKG_NS_LABEL   = "k8spkg.mgoltzsche.github.com/namespaces"
)

type notFoundError struct {
	error
}

func IsNotFound(err error) bool {
	_, ok := err.(notFoundError)
	return ok
}

type PackageManager struct {
	namespace     string
	client        client.K8sClient
	installedApps *AppRepo
	resourceTypes []*client.APIResourceType
}

func NewPackageManager(client client.K8sClient, namespace string) *PackageManager {
	return &PackageManager{namespace, client, NewAppRepo(client), nil}
}

func (m *PackageManager) List(ctx context.Context, namespace string) (apps []*App, err error) {
	return m.installedApps.GetAll(ctx, namespace)
}

func (m *PackageManager) labelSelector(pkgName string) []string {
	return []string{PKG_NAME_LABEL + "=" + pkgName}
}

func (m *PackageManager) Status(ctx context.Context, pkg *K8sPackage) (err error) {
	return m.await(ctx, pkg.Name, pkg.Resources, status.RolloutConditions)
}

func (m *PackageManager) await(ctx context.Context, appName string, resources resource.K8sResourceList, conditions map[string]status.Condition) (err error) {
	ctx, cancel := context.WithCancel(ctx)
	reg := status.NewResourceTracker(resources, conditions)
	ready := false
	found := false
	deadline, hasDeadline := ctx.Deadline()
	watchCtx := ctx
	if hasDeadline {
		watchCtx, _ = context.WithDeadline(ctx, deadline.Add(-7*time.Second))
	}
	ch := m.watch(watchCtx, appName, resources.Refs())
findLoop:
	for {
		for evt := range ch {
			if evt.Error == nil {
				if s, changed := reg.Update(evt.Resource); changed {
					msg := fmt.Sprintf("%s/%s: %s", strings.ToLower(evt.Resource.Kind()), evt.Resource.Name(), s.Description)
					if s.Status {
						logrus.Info(msg)
						if ready = reg.Ready(); ready {
							cancel()
						}
					} else {
						logrus.Warn(msg)
					}
					if reg.Found() && !found {
						found = true
						if !ready {
							if podCh := m.watchPods(watchCtx, reg); len(podCh) > 0 {
								ch = resource.WatchEventUnion(append(podCh, ch))
								continue findLoop
							}
						}
					}
				}
			} else if err == nil && !ready {
				err = evt.Error
				cancel()
			}
		}
		break
	}
	if !reg.Ready() {
		var failedPods resource.K8sResourceList
		failedObj := 0
		for _, o := range reg.Status() {
			if !o.Status.Status {
				failedObj++
				logrus.Errorf("%s/%s: %s", strings.ToLower(o.Resource.Kind()), o.Resource.Name(), o.Status.Description)
				if o.Resource.Kind() == "Pod" {
					failedPods = append(failedPods, o.Resource)
				}
			}
		}
		if err == nil {
			err = errors.Errorf("%d resources did not meet condition", failedObj)
		}
		if len(failedPods) > 0 {
			// TODO: group common pods and print logs of a pod per group
			pod := failedPods[0]
			for _, cs := range pod.ContainerStatuses() {
				if cs.ExitCode > 0 {
					if e := m.client.ContainerLogs(ctx, pod.Namespace(), pod.Name(), cs.Name, os.Stderr); err != nil {
						logrus.Warn(e)
					}
					break
				}
			}
		}
	}
	return
}

func (m *PackageManager) watchPods(ctx context.Context, t *status.ResourceTracker) (ch []<-chan resource.ResourceEvent) {
	status := t.Status()
	podNs := map[string]bool{}
	for _, o := range status {
		if o.Resource.Kind() == "Pod" {
			podNs[o.Resource.Namespace()] = true
		}
	}
	for _, o := range status {
		if !o.Status.Status && (o.Resource.Kind() == "Deployment" || o.Resource.Kind() == "DaemonSet") && !podNs[o.Resource.Namespace()] {
			ch = append(ch, m.client.Watch(ctx, "Pod", o.Resource.Namespace(), o.Resource.SelectorMatchLabels()))
		}
	}
	return
}

func (m *PackageManager) watch(ctx context.Context, appName string, resources resource.K8sResourceRefList) <-chan resource.ResourceEvent {
	ch := []<-chan resource.ResourceEvent{}
	pkgSelector := m.labelSelector(appName)
	for _, byNs := range resources.GroupByNamespace() {
		if byNs.Key == "" {
			byNs.Key = m.namespace
		}
		for _, byKind := range byNs.Resources.GroupByKind() {
			ch = append(ch, m.client.Watch(ctx, byKind.Key, byNs.Key, pkgSelector))
		}
	}
	return resource.WatchEventUnion(ch)
}

func (m *PackageManager) Apply(ctx context.Context, pkg *K8sPackage, prune bool) (err error) {
	logrus.Infof("Applying package %s...", pkg.Name)
	app := App{
		Name:      pkg.Name,
		Namespace: m.namespace,
		Resources: pkg.Resources.Refs(),
	}
	if err = m.installedApps.Put(ctx, &app); err != nil {
		return
	}
	pkgLabel := []string{PKG_NAME_LABEL + "=" + pkg.Name}
	applied, err := m.client.Apply(ctx, app.Namespace, pkg.Resources, prune, pkgLabel)
	if err == nil {
		err = m.await(ctx, pkg.Name, applied, status.RolloutConditions)
	}
	if err == nil {
		logrus.Infof("Applied %s successfully", pkg.Name)
	}
	return errors.Wrapf(err, "apply package %s", pkg.Name)
}

func (m *PackageManager) Delete(ctx context.Context, name string) (err error) {
	app, err := m.installedApps.Get(ctx, m.namespace, name)
	if err == nil {
		logrus.Infof("Deleting %s...", name)
		resources := app.Resources
		sort.Sort(reverseResources(resources))
		if err = m.deleteResources(ctx, app.Name, resources); err == nil {
			if err = m.installedApps.Delete(ctx, app); err == nil {
				logrus.Infof("Deleted %s", name)
			}
		}
	}
	return errors.Wrapf(err, "delete package %s", name)
}

func (m *PackageManager) DeleteResources(ctx context.Context, obj resource.K8sResourceRefList) (err error) {
	return m.deleteResources(ctx, "", obj)
}

func (m *PackageManager) deleteResources(ctx context.Context, appName string, obj resource.K8sResourceRefList) (err error) {
	if err = m.client.Delete(ctx, m.namespace, obj); err == nil {
		m.client.AwaitDeletion(ctx, m.namespace, obj)
	}
	return
}

type reverseResources resource.K8sResourceRefList

// Less reports whether the element with
// index i should sort before the element with index j.
func (r reverseResources) Less(i, j int) bool {
	return i < j
}
func (r reverseResources) Len() int {
	return len(r)
}

// Swap swaps the elements with indexes i and j.
func (r reverseResources) Swap(i, j int) {
	tmp := r[i]
	r[i] = r[j]
	r[j] = tmp
}
