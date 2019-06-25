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

func (m *PackageManager) State(pkgName string) (objects []*model.K8sObject, err error) {
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

func (m *PackageManager) Apply(objects []*model.K8sObject, prune bool) (err error) {
	// TODO: store source URL as commonLabel as well and provide an option to
	//       require source equality to do a k8s object update to make
	//       sure nobody accidentally deletes k8s objects when reusing an existing package name
	pkgName, err := packageName(objects)
	if err != nil {
		return
	}
	log := logrus.WithField("pkg", pkgName)
	log.Infof("Applying package %s", pkgName)
	reader, errc := manifestReader(objects)
	pkgLabel := model.PKG_LABEL + "=" + pkgName
	args := []string{"apply", "--wait=true", "--timeout=2m", "-f", "-", "-l", pkgLabel}
	if prune {
		args = append(args, "--prune")
	}
	err = runKubectl(log, reader, args)
	reader.Close()
	if e := <-errc; e != nil && err == nil {
		err = e
	}
	if err == nil {
		if err = m.awaitRollout(objects, log); err == nil {
			err = m.awaitAvailability(objects, log)
		}
	}
	return errors.Wrapf(err, "apply package %s", pkgName)
}

func packageName(objects []*model.K8sObject) (pkgName string, err error) {
	if len(objects) == 0 {
		return "", errors.New("no objects provided")
	}
	for _, o := range objects {
		// Sets k8s's part-of and managed-by labels.
		//m.Labels["app.kubernetes.io/managed-by"] = "k8spkg"
		packageName := o.Labels()[model.PKG_LABEL]
		if packageName == "" {
			return "", errors.Errorf("%s/%s declares no package name label %s", o.Kind, o.Name, model.PKG_LABEL)
		}
		if pkgName == "" {
			pkgName = packageName
		} else if pkgName != packageName {
			return "", errors.Errorf("more than one package referenced within the provided objects: %s, %s", pkgName, packageName)
		}
	}
	return
}

func (m *PackageManager) awaitRollout(objects []*model.K8sObject, log *logrus.Entry) (err error) {
	for _, o := range objects {
		kind := o.Kind
		if kind == "Deployment" || kind == "DaemonSet" || kind == "StatefulSet" {
			args := []string{"rollout", "status", "-w", "--timeout=2m"}
			if o.Namespace != "" {
				args = append(args, "-n", o.Namespace)
			}
			args = append(args, strings.ToLower(kind)+"/"+o.Name)
			if err = runKubectl(log, nil, args); err != nil {
				return
			}
		}
	}
	return
}

func (m *PackageManager) awaitAvailability(objects []*model.K8sObject, log *logrus.Entry) (err error) {
	obj := filter(objects, func(o *model.K8sObject) bool {
		return o.Kind == "Deployment" || o.Kind == "APIService" // TODO: add more
	})
	if len(obj) == 0 {
		return nil
	}
	log.Infof("Waiting for %d components to become available...", len(obj))
	return kubectlWait(log, obj, "condition=available")
}

func kubectlWait(log *logrus.Entry, obj []*model.K8sObject, forExpr string) (err error) {
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
	if e := runKubectl(log, nil, append(args, names...)); e != nil && err == nil {
		err = e
	}
	return
}

func filter(obj []*model.K8sObject, filter func(*model.K8sObject) bool) (filtered []*model.K8sObject) {
	for _, o := range obj {
		if filter(o) {
			filtered = append(filtered, o)
		}
	}
	return
}

func groupByNamespace(obj []*model.K8sObject) (nsMap map[string][]*model.K8sObject, nsOrder []string) {
	nsMap = map[string][]*model.K8sObject{}
	nsOrder = []string{}
	for _, o := range obj {
		l := nsMap[o.Namespace]
		if l == nil {
			nsOrder = append(nsOrder, o.Namespace)
		}
		nsMap[o.Namespace] = append(l, o)
	}
	return
}

func (m *PackageManager) clearResourceTypeCache() {
	m.resourceTypes = &ApiResourceTypes{}
}

func (m *PackageManager) Delete(pkgName string) (err error) {
	log := logrus.WithField("pkg", pkgName)
	o, err := m.State(pkgName)
	if err != nil {
		return
	}
	if len(o) == 0 {
		log.Warn("Package not found")
		return nil
	}

	m.clearResourceTypeCache()
	err = m.deleteObjects(log, o)
	if err != nil {
		// Workaround to exit successfully in case `kubectl wait` did not find an already deleted resource.
		// This should be solved within kubectl so that it does not exit with an error when waiting for deletion of a deleted resource.
		if o, e := m.State(pkgName); e == nil {
			if len(o) == 0 {
				err = nil
			} else {
				err = errors.Errorf("leftover resources: %s", strings.Join(names(o), ", "))
			}
		}
	}
	return errors.Wrapf(err, "delete package %s", pkgName)
}

func (m *PackageManager) deleteObjects(log *logrus.Entry, obj []*model.K8sObject) (err error) {
	fqnMap := map[string]bool{}
	crds := filter(obj, func(o *model.K8sObject) bool { return o.Kind == "CustomResourceDefinition" })
	mapFqns(crds, fqnMap)
	crdMap := crdGvkMap(crds)
	crdRes := filter(obj, func(o *model.K8sObject) bool { return crdMap[o.Gvk()] })
	mapFqns(crdRes, fqnMap)
	namespaced := filter(obj, func(o *model.K8sObject) bool { return !fqnMap[o.ID()] && o.Namespace != "" })
	mapFqns(namespaced, fqnMap)
	other := filter(obj, func(o *model.K8sObject) bool { return !fqnMap[o.ID()] })

	deletionOrder := [][]*model.K8sObject{
		crdRes,
		namespaced,
		other,
		crds,
	}

	for _, items := range deletionOrder {
		nsMap, nsOrder := groupByNamespace(items)
		for _, ns := range nsOrder {
			nonContained := filter(nsMap[ns], isNoContainedObject)
			if e := deleteObjectNames(log, ns, names(nonContained)); e != nil && err == nil {
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
	return runKubectl(log, nil, append(args, names...))
}

func isNoContainedObject(o *model.K8sObject) bool {
	return len(o.OwnerReferences()) == 0
}

func crdGvkMap(crds []*model.K8sObject) (m map[string]bool) {
	m = map[string]bool{}
	for _, o := range crds {
		m[o.CrdGvk()] = true
	}
	return
}

func mapFqns(obj []*model.K8sObject, fqns map[string]bool) {
	for _, o := range obj {
		fqns[o.ID()] = true
	}
}

func names(obj []*model.K8sObject) (names []string) {
	names = make([]string, len(obj))
	for i, o := range obj {
		names[i] = strings.ToLower(o.Kind) + "/" + o.Name
	}
	return
}

func manifestReader(objects []*model.K8sObject) (io.ReadCloser, chan error) {
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

func runKubectl(log *logrus.Entry, stdin io.Reader, args []string) (err error) {
	c := exec.Command("kubectl", args...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Stdin = stdin
	log.Debugf("Running %+v", c.Args)
	err = c.Run()
	return errors.Wrapf(err, "%+v", c.Args)
}
