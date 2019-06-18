package state

import (
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/mgoltzsche/k8spkg/pkg/model"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type PackageManager struct {
	resourceTypes *ApiResourceTypes
}

func NewPackageManager() *PackageManager {
	return &PackageManager{&ApiResourceTypes{}}
}

func (m *PackageManager) State(pkgName string) (objects []model.K8sObject, err error) {
	// TODO: get all api-resources: kubectl api-resources -o name --verbs delete [--namespaced]
	// get all resources: kubectl get deploy,pod,... --all-namespaces -l app.kubernetes.io/part-of=cert-manager -o yaml
	// deletion order:
	// - crd resources
	// - namespaced resources
	// - global resources
	resourceTypes, err := m.resourceTypes.All()
	if err != nil {
		return
	}
	types := strings.Join(resourceTypes, ",")
	reader, writer := io.Pipe()
	defer func() {
		if e := reader.Close(); e != nil && err == nil {
			err = e
		}
		err = errors.Wrap(err, "read 'kubectl get' output")
	}()
	errc := make(chan error)
	go func() {
		var e error
		objects, e = model.K8sObjectsFromReader(reader)
		errc <- e
		writer.CloseWithError(e)
	}()
	c := exec.Command("kubectl", "get", types, "--all-namespaces", "-l", model.PKG_LABEL+"="+pkgName, "-o", "yaml")
	c.Stdout = writer
	c.Stderr = os.Stderr
	err = c.Run()
	writer.Close()
	if e := <-errc; e != nil && err == nil {
		err = e
	}
	err = errors.WithMessagef(err, "%+v", c.Args)
	return
}

func (m *PackageManager) Apply(pkg *model.K8sPackage) (err error) {
	log := logrus.WithField("pkg", pkg.Name)
	log.Infof("Applying package")
	for _, o := range pkg.Objects {
		m := o.Metadata()
		// Sets k8s's part-of and managed-by labels.
		m.Labels["app.kubernetes.io/managed-by"] = "k8spkg"
		m.Labels[model.PKG_LABEL] = pkg.Name
		o.SetMetadata(m)
	}
	reader, errc := manifestReader(pkg.Objects)
	pkgLabel := model.PKG_LABEL + "=" + pkg.Name
	args := []string{"apply", "--wait=true", "--prune", "-f", "-", "-l", pkgLabel}
	c := exec.Command("kubectl", args...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Stdin = reader
	log.Debugf("Running %+v", c.Args)
	err = c.Run()
	reader.Close()
	if e := <-errc; e != nil && err == nil {
		err = e
	}
	if err == nil {
		if err = m.AwaitRollout(pkg); err == nil {
			err = m.AwaitAvailability(pkg)
		}
	}
	return errors.Wrapf(err, "apply package %q", pkg.Name)
}

func (m *PackageManager) AwaitRollout(pkg *model.K8sPackage) (err error) {
	log := logrus.WithField("pkg", pkg.Name)
	for _, o := range pkg.Objects {
		kind := o.Kind()
		if kind == "Deployment" || kind == "DaemonSet" || kind == "StatefulSet" {
			meta := o.Metadata()
			args := []string{"rollout", "status", "-w", "--timeout=2m"}
			if meta.Namespace != "" {
				args = append(args, "-n", meta.Namespace)
			}
			args = append(args, strings.ToLower(kind)+"/"+meta.Name)
			if err = runKubectl(log, args); err != nil {
				return
			}
		}
	}
	return
}

func (m *PackageManager) AwaitAvailability(pkg *model.K8sPackage) (err error) {
	log := logrus.WithField("pkg", pkg.Name)
	obj := filter(k8sitems(pkg.Objects), func(o *k8sitem) bool {
		return o.kind == "Deployment" || o.kind == "APIService" // TODO: add more
	})
	if len(obj) == 0 {
		return nil
	}
	log.Infof("Waiting for %d components to become available...", len(obj))
	return kubectlWait(log, obj, "condition=available")
}

func kubectlWait(log *logrus.Entry, obj []*k8sitem, forExpr string) (err error) {
	nsMap, nsOrder := groupByNamespace(obj)
	for _, ns := range nsOrder {
		if e := kubectlWaitNames(log, ns, names(nsMap[ns]), forExpr); e != nil {
			err = e
		}
	}
	return
}

func kubectlWaitNames(log *logrus.Entry, ns string, names []string, forExpr string) (err error) {
	if len(names) == 0 {
		return
	}
	args := []string{"wait", "--for", forExpr, "--timeout=2m"}
	if ns != "" {
		args = append(args, "-n", ns)
	}
	if e := runKubectl(log, append(args, names...)); e != nil && err == nil {
		err = e
	}
	return
}

func filter(obj []*k8sitem, filter func(*k8sitem) bool) (filtered []*k8sitem) {
	for _, o := range obj {
		if filter(o) {
			filtered = append(filtered, o)
		}
	}
	return
}

func groupByNamespace(obj []*k8sitem) (nsMap map[string][]*k8sitem, nsOrder []string) {
	nsMap = map[string][]*k8sitem{}
	nsOrder = []string{}
	for _, o := range obj {
		l := nsMap[o.ns]
		if l == nil {
			nsOrder = append(nsOrder, o.ns)
		}
		nsMap[o.ns] = append(l, o)
	}
	return
}

/*func (m *PackageManager) Delete(pkg *model.K8sPackage) (err error) {
	if pkg.Name == "" {
		return errors.New("no package name provided")
	}
	log := logrus.WithField("pkg", pkg.Name)
	log.Infof("Deleting package %q", pkg.Name)
	// Delete objects contained within merged manifest yaml
	err = m.deleteObjects(log, pkg.Objects)

	// Delete objects belonging to another package version with the same name
	err2 := m.DeleteName(log, pkg.Name)
	if err2 == nil {
		err = nil
	} else {
		err = err2
	}
	return errors.Wrapf(err, "delete package %q", pkg.Name)
}*/

func (m *PackageManager) clearResourceTypeCache() {
	m.resourceTypes = &ApiResourceTypes{}
}

func (m *PackageManager) Delete(pkgName string) (err error) {
	log := logrus.WithField("pkg", pkgName)
	o, err := m.State(pkgName) // TODO: make sure all resources are returned (currently only deploy, service)
	if err != nil {
		return
	}
	if len(o) == 0 {
		log.Warn("Package not found")
		return nil
	}

	obj := k8sitems(o)
	m.clearResourceTypeCache()
	err = m.deleteObjects(log, obj)
	if err != nil {
		// Workaround to exit successfully in case `kubectl wait` did not find an already deleted resource.
		// This should be solved within kubectl so that it does not exit with an error when waiting for deletion of a deleted resource.
		if o, e := m.State(pkgName); e == nil {
			if len(o) == 0 {
				err = nil
			} else {
				err = errors.Errorf("leftover resources: %s", strings.Join(names(obj), ", "))
			}
		}
	}
	return errors.Wrapf(err, "delete package %q", pkgName)
}

type k8sitem struct {
	fqn        string
	apiVersion string
	ns         string
	kind       string
	name       string
	obj        model.K8sObject
}

func k8sitems(obj []model.K8sObject) (l []*k8sitem) {
	l = make([]*k8sitem, len(obj))
	for i, o := range obj {
		m := o.Metadata()
		l[i] = &k8sitem{fqn(o), o.ApiVersion(), m.Namespace, o.Kind(), m.Name, o}
	}
	return
}

func (m *PackageManager) deleteObjects(log *logrus.Entry, obj []*k8sitem) (err error) {
	// TODO: wait for pod deletion reliably (add commonLabel with kustomize that is applied to pods within a deployment as well)
	// TODO: maybe don't delete pods if they are owned by a deployment but wait for pods to be deleted after the deployment has been deleted
	fqnMap := map[string]bool{}
	crds := filter(obj, func(o *k8sitem) bool { return o.kind == "CustomResourceDefinition" })
	mapFqns(crds, fqnMap)
	crdMap := crdGvkMap(crds)
	crdRes := filter(obj, func(o *k8sitem) bool { return crdMap[o.apiVersion+"/"+o.kind] })
	mapFqns(crdRes, fqnMap)
	namespaced := filter(obj, func(o *k8sitem) bool { return !fqnMap[o.fqn] && o.ns != "" })
	mapFqns(namespaced, fqnMap)
	other := filter(obj, func(o *k8sitem) bool { return !fqnMap[o.fqn] })

	deletionOrder := [][]*k8sitem{
		crdRes,
		namespaced,
		other,
		crds,
	}

	for _, items := range deletionOrder {
		nsMap, nsOrder := groupByNamespace(items)
		// Delete namespaced resources
		for _, ns := range nsOrder {
			if e := deleteObjectNames(log, ns, names(nsMap[ns])); e != nil && err == nil {
				err = e
			}
		}
		for _, ns := range nsOrder {
			if e := kubectlWaitNames(log, ns, names(nsMap[ns]), "delete"); e != nil && err == nil {
				err = waitError(e)
			}
		}
	}
	return
}

type waitError error

func deleteObjectNames(log *logrus.Entry, ns string, names []string) (err error) {
	if len(names) == 0 {
		return
	}
	args := []string{"delete", "--wait=true", "--timeout=2m", "--cascade=true", "--ignore-not-found=true"}
	if ns != "" {
		args = append(args, "-n", ns)
	}
	return runKubectl(log, append(args, names...))
}

func crdGvkMap(crds []*k8sitem) (m map[string]bool) {
	m = map[string]bool{}
	for _, o := range crds {
		group := o.obj.GetString("spec.group")
		version := o.obj.GetString("spec.version")
		kind := o.obj.GetString("spec.names.kind")
		m[group+"/"+version+"/"+kind] = true
	}
	return
}

func mapFqns(obj []*k8sitem, fqns map[string]bool) {
	for _, o := range obj {
		fqns[o.fqn] = true
	}
}

func fqn(o model.K8sObject) string {
	m := o.Metadata()
	return m.Namespace + "/" + o.ApiVersion() + "/" + o.Kind() + "/" + m.Name
}

func names(obj []*k8sitem) (names []string) {
	names = make([]string, len(obj))
	for i, o := range obj {
		names[i] = strings.ToLower(o.kind) + "/" + o.name
	}
	return
}

/*
// IsLessThan returns true if self is less than the argument.
func (x Gvk) IsLessThan(o Gvk) bool {
	indexI := typeOrders[x.Kind]
	indexJ := typeOrders[o.Kind]
	if indexI != indexJ {
		return indexI < indexJ
	}
	return x.String() < o.String()
}

func sortPackages(obj []model.K8sObject) {
	sort.Sli
}*/

/*func (m *PackageManager) awaitDeletion(log *logrus.Entry, obj []model.K8sObject) (err error) {
	// TODO: make sure all objects are deleted
	nsMap, nsOrder := groupByNamespace(obj, func(obj model.K8sObject) bool {
		return true
	})
	if len(nsOrder) > 0 {
		log.Info("Waiting for resource deletion...")
	}
	for _, ns := range nsOrder {
		waitArgs := []string{"wait", "--for", "delete", "--timeout=2m"}
		if ns != "" {
			waitArgs = append(waitArgs, "-n", ns)
		}
		for _, o := range nsMap[ns] {
			m := o.Metadata()
			kind := strings.ToLower(o.Kind())
			waitArgs = append(waitArgs, kind+"/"+m.Name)
		}
		if e := runKubectl(log, waitArgs...); e != nil && err == nil {
			err = errors.New("failed waiting for all resources to be deleted")
		}
	}
	return
}*/

func manifestReader(objects []model.K8sObject) (io.ReadCloser, chan error) {
	reader, writer := io.Pipe()
	errc := make(chan error)
	go func() {
		var err error
		for _, o := range objects {
			if err = o.WriteYaml(writer); err != nil {
				break
			}
		}
		writer.CloseWithError(err)
		errc <- err
	}()
	return reader, errc
}

func runKubectl(log *logrus.Entry, args []string) (err error) {
	c := exec.Command("kubectl", args...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	log.Debugf("Running %+v", c.Args)
	err = c.Run()
	return errors.Wrapf(err, "%+v", c.Args)
}
