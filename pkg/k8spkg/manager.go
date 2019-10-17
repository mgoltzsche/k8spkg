package k8spkg

import (
	"context"
	"fmt"
	"sort"

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
	return m.await(ctx, pkg.Name, pkg.Resources.Refs(), status.RolloutConditions)
}

func (m *PackageManager) await(ctx context.Context, appName string, resources resource.K8sResourceRefList, conditions map[string]status.Condition) (err error) {
	ctx, cancel := context.WithCancel(ctx)
	pkgSelector := m.labelSelector(appName)
	reg := status.NewResourceTracker(resources, conditions)
	ready := false
	found := false
	ch, podNs := m.watch(ctx, appName, resources)
	for evt := range ch {
		if evt.Error == nil {
			if s, changed := reg.Update(evt.Resource); changed {
				msg := fmt.Sprintf("%s/%s: %s", evt.Resource.Kind(), evt.Resource.Name(), s.Description)
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
						for _, ns := range podNs {
							ch = m.client.Watch(ctx, "Pod", ns, pkgSelector)
						}
					}
				}
			}
		} else if err == nil && !ready {
			err = evt.Error
			cancel()
		}
	}
	failedObj := 0
	for _, o := range reg.Status() {
		if o.Status != nil && !o.Status.Status && err == nil {
			failedObj++
		}
		if o.Status != nil && !o.Status.Status {
			logrus.Errorf("%s/%s: %s", o.Resource.Kind(), o.Resource.Name(), o.Status.Description)
		}
	}
	if failedObj > 0 && err == nil {
		err = errors.Errorf("%d resources did not meet condition", failedObj)
	}
	return
}

func (m *PackageManager) watch(ctx context.Context, appName string, resources resource.K8sResourceRefList) (c <-chan resource.ResourceEvent, unwatchedPodNs []string) {
	ch := []<-chan resource.ResourceEvent{}
	pkgSelector := m.labelSelector(appName)
	ctrlNamespaces := map[string]bool{}
	podNamespaces := map[string]bool{}
	for _, byNs := range resources.GroupByNamespace() {
		if byNs.Key == "" {
			byNs.Key = m.namespace
		}
		for _, byKind := range byNs.Resources.GroupByKind() {
			ch = append(ch, m.client.Watch(ctx, byKind.Key, byNs.Key, pkgSelector))
			if byKind.Key == "Deployment" || byKind.Key == "DaemonSet" || byKind.Key == "Job" {
				ctrlNamespaces[byNs.Key] = true
			} else if byKind.Key == "Pod" {
				podNamespaces[byNs.Key] = true
			}
		}
	}
	// TODO: trigger implicit pod watch for ns only after Deployment/DaemonSet/Job has received their first negative status.
	for ns := range ctrlNamespaces {
		if !podNamespaces[ns] {
			unwatchedPodNs = append(unwatchedPodNs, ns)
		}
	}
	return resource.WatchEventUnion(ch), unwatchedPodNs
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
	_, err = m.client.Apply(ctx, app.Namespace, pkg.Resources, prune, pkgLabel)
	if err == nil {
		err = m.await(ctx, pkg.Name, pkg.Resources.Refs(), status.RolloutConditions)
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
