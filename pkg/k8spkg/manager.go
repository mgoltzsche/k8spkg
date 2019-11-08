package k8spkg

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/mgoltzsche/k8spkg/pkg/client"
	"github.com/mgoltzsche/k8spkg/pkg/resource"
	"github.com/mgoltzsche/k8spkg/pkg/status"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	// See https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/
	PKG_NAME_LABEL = "app.kubernetes.io/part-of"
	PKG_NS_LABEL   = "k8spkg.mgoltzsche.github.com/namespace"
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

func (m *PackageManager) List(ctx context.Context, namespace string) <-chan AppEvent {
	return m.installedApps.GetAll(ctx, namespace)
}

func (m *PackageManager) labelSelector(pkgName string) []string {
	return []string{PKG_NAME_LABEL + "=" + pkgName}
}

func (m *PackageManager) Status(ctx context.Context, pkg *K8sPackage) (err error) {
	return m.await(ctx, pkg.Name, pkg.Resources, status.RolloutConditions)
}

func (m *PackageManager) await(ctx context.Context, appName string, resources resource.K8sResourceList, conditions map[string]status.Condition) (err error) {
	var conditional []resource.K8sResourceRef
	refs := resources.Refs()
	for _, ref := range refs {
		if conditions[ref.Kind()] != nil {
			conditional = append(conditional, ref)
		}
	}
	watchCtx, cancel := context.WithCancel(ctx)
	evts := Events(watchCtx, refs, m.client)
	resEvts := m.watch(watchCtx, appName, conditional)
	statusEvts := status.Emitter(resEvts, conditions)
	tracker := status.NewTracker(conditional, statusEvts)
	result := tracker.Result()
	changes := tracker.Changes()
	ready := tracker.Ready()
	go tracker.Run()
	for {
		if changes == nil && evts == nil && ready == nil {
			break
		}
		select {
		case evt, ok := <-changes:
			// status update
			if !ok {
				changes = nil
				cancel()
				continue
			}
			if evt.Err == nil {
				msg := fmt.Sprintf("%s/%s: %s", strings.ToLower(evt.Resource.Kind()), evt.Resource.Name(), evt.Status.Description)
				if evt.Status.Status {
					logrus.Info(msg)
				} else {
					logrus.Warn(msg)
				}
			} else if err == nil {
				if errors.Cause(evt.Err) != context.Canceled {
					err = evt.Err
				}
				cancel()
			}
		case evt, ok := <-evts:
			// event
			if !ok {
				evts = nil
				continue
			}
			if evt.Type != "Normal" {
				// TODO: don't handle events that belong to an old replica!
				//   (Then the BackOff event could be used to reliably fail the deployment fast.)
				//   Could be achieved by getting the latest replicaset version
				//   and compare its pod name prefix with the failed pod
				msg := fmt.Sprintf("%s/%s: %s", strings.ToLower(evt.InvolvedObject.Kind()), evt.InvolvedObject.Name(), evt.Reason)
				if evt.Message != "" {
					msg += ": " + evt.Message
				}
				if evt.Reason == "BackOff" {
					logrus.Error(msg)
					container := containerNameFromFieldPath(evt.InvolvedFieldPath)
					if evt.InvolvedObject.Kind() == "Pod" && container != "" {
						m.logPodError(watchCtx, evt.InvolvedObject, container)
					}
				} else {
					logrus.Warn(msg)
				}
			}
		case _, ok := <-ready:
			// all tracked resources ready - cancel watches
			if !ok {
				ready = nil
				continue
			}
			cancel()
		}
	}
	summary := <-result
	for _, o := range summary.Resources {
		if !o.Status.Status {
			logrus.Errorf("%s/%s: %s", strings.ToLower(o.Resource.Kind()), o.Resource.Name(), o.Status.Description)
		}
	}
	if err == nil && !summary.Ready {
		err = errors.New("resources did not meet condition")
	}
	if e := ctx.Err(); e != nil {
		err = e
	}
	return
}

var containerNamePattern = regexp.MustCompile("^spec\\.containers\\{([^}]+)}$")

func containerNameFromFieldPath(fieldPath string) string {
	m := containerNamePattern.FindStringSubmatch(fieldPath)
	if len(m) == 2 {
		return m[1]
	}
	return ""
}

func (m *PackageManager) logPodError(ctx context.Context, pod resource.K8sResourceRef, container string) {
	writer := newChanWriter()
	go func() {
		if e := m.client.ContainerLogs(ctx, pod.Namespace(), pod.Name(), container, false, true, writer); e != nil {
			logrus.Debug(e)
		}
		writer.Close()
	}()
	ctxLogged := false
	for logLine := range writer.Chan() {
		if !ctxLogged {
			ctxLogged = true
			logrus.Errorf("pod/%s: container %s logs:", pod.Name(), container)
		}
		logrus.Error(" " + logLine)
	}
}

func (m *PackageManager) watch(ctx context.Context, appName string, resources resource.K8sResourceRefList) <-chan resource.ResourceEvent {
	pkgSelector := m.labelSelector(appName)
	evts := make(chan resource.ResourceEvent)
	wg := sync.WaitGroup{}
	for _, byNs := range resources.GroupByNamespace() {
		if byNs.Key == "" {
			byNs.Key = m.namespace
		}
		for _, byKind := range byNs.Resources.GroupByKind() {
			wg.Add(1)
			go func(kind, ns string) {
				for evt := range m.client.Watch(ctx, kind, ns, pkgSelector, false) {
					evts <- evt
				}
				wg.Done()
			}(byKind.Key, byNs.Key)
		}
	}
	go func() {
		wg.Wait()
		close(evts)
	}()
	return evts
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
	// TODO: detect which resources changed or have been created
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
		if err = m.deleteResources(ctx, resources); err == nil {
			if err = m.installedApps.Delete(ctx, app); err == nil {
				logrus.Infof("Deleted %s", name)
			}
		}
	}
	return errors.Wrapf(err, "delete package %s", name)
}

func (m *PackageManager) DeleteResources(ctx context.Context, obj resource.K8sResourceRefList) (err error) {
	return m.deleteResources(ctx, obj)
}

func (m *PackageManager) deleteResources(ctx context.Context, obj resource.K8sResourceRefList) (err error) {
	if err = m.client.Delete(ctx, m.namespace, obj); err == nil {
		err = m.client.AwaitDeletion(ctx, m.namespace, obj)
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
