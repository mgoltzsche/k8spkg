package k8spkg

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/mgoltzsche/k8spkg/pkg/model"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	// See https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/
	PKG_NAME_LABEL = "app.kubernetes.io/part-of"
	PKG_NS_LABEL   = "k8spkg.mgoltzsche.github.com/namespaces"
)

type PackageManager struct {
	kubeconfigFile string
	resourceTypes  []*APIResourceType
}

func NewPackageManager(kubeconfigFile string) *PackageManager {
	return &PackageManager{kubeconfigFile, nil}
}

func (m *PackageManager) clearResourceTypeCache() {
	m.resourceTypes = nil
}

type notFoundError struct {
	error
}

func IsNotFound(err error) bool {
	_, ok := err.(notFoundError)
	return ok
}

func (m *PackageManager) State(ctx context.Context, namespace, pkgName string) (pkg *K8sPackage, err error) {
	if pkgName == "" {
		return nil, errors.New("no package name provided")
	}
	objects, err := m.objects(ctx, false, namespace, pkgName)
	if err != nil {
		return
	}
	if len(objects) == 0 {
		return nil, &notFoundError{errors.Errorf("package %q not found", pkgName)}
	}
	infos, _ := PackageInfosFromObjects(objects)
	if len(infos) != 1 {
		panic("len(pkgInfos) != 1")
	}
	pkg = &K8sPackage{infos[0], objects}
	namespaces := pkg.Namespaces
	namespace = detectNamespace(namespace, objects)
	if len(namespaces) == 0 || (len(namespaces) == 1 && namespaces[0] == namespace) {
		return
	}

	// Fetch objects from namespaces referenced in loaded object's label
	resTypes, err := m.apiResources(ctx)
	if err != nil {
		return
	}
	namespacedTypeNames := make([]string, 0, len(resTypes))
	for _, t := range resTypes {
		if t.Namespaced {
			namespacedTypeNames = append(namespacedTypeNames, t.FullName())
		}
	}
	for _, ns := range namespaces {
		if ns != namespace {
			if objects, err = m.kubectlGetPkg(ctx, namespacedTypeNames, false, ns, pkgName); err != nil {
				return
			}
			pkg.Objects = append(pkg.Objects, objects...)
		}
	}
	return
}

func (m *PackageManager) apiResources(ctx context.Context) (t []*APIResourceType, err error) {
	if m.resourceTypes == nil {
		m.resourceTypes, err = LoadAPIResourceTypes(ctx, m.kubeconfigFile)
	}
	return m.resourceTypes, err
}

func (m *PackageManager) objects(ctx context.Context, allNamespaces bool, namespace, pkgName string) (objects []*model.K8sObject, err error) {
	resTypes, err := m.apiResources(ctx)
	if err != nil {
		return
	}
	typeNames := make([]string, len(resTypes))
	for i, t := range resTypes {
		typeNames[i] = t.FullName()
	}
	return m.kubectlGetPkg(ctx, typeNames, allNamespaces, namespace, pkgName)
}

func detectNamespace(namespace string, objects []*model.K8sObject) string {
	if namespace == "" {
		for _, o := range objects {
			if o.Namespace != "" {
				return o.Namespace
			}
		}
	}
	return namespace
}

func (m *PackageManager) List(ctx context.Context, allNamespaces bool, namespace string) (pkgs []*PackageInfo, err error) {
	// TODO: fetch necessary values only instead of whole objects
	obj, err := m.objects(ctx, allNamespaces, namespace, "")
	if err != nil {
		return
	}
	pkgs, _ = PackageInfosFromObjects(obj)
	return
}

func (m *PackageManager) kubectlGetPkg(ctx context.Context, types []string, allNamespaces bool, namespace, pkgName string) (objects []*model.K8sObject, err error) {
	typeCsv := strings.Join(types, ",")
	args := []string{typeCsv}
	if pkgName != "" {
		args = append(args, "-l", PKG_NAME_LABEL+"="+pkgName)
	}
	if allNamespaces && namespace != "" {
		return nil, errors.Errorf("invalid arguments: allNamespaces=true and namespace set")
	}
	if allNamespaces {
		args = append(args, "--all-namespaces")
	}
	return m.kubectlGet(ctx, namespace, args)
}

func (m *PackageManager) kubectlGet(ctx context.Context, namespace string, args []string) (objects []*model.K8sObject, err error) {
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
		objects, e = model.FromReader(reader)
		errc <- e
		writer.CloseWithError(e)
	}()
	c := newKubectlCmd(ctx, m.kubeconfigFile)
	c.Stdout = writer
	c.Stderr = os.Stderr
	args = append([]string{"get", "-o", "yaml"}, args...)
	if namespace != "" {
		args = append(args, "-n", namespace)
	}
	err = c.Run(args...)
	writer.Close()
	if e := <-errc; e != nil && err == nil {
		err = e
	}
	return
}

func (m *PackageManager) Apply(ctx context.Context, pkg *K8sPackage, prune bool) (err error) {
	logrus.Infof("Applying package %s", pkg.Name)
	reader := manifestReader(pkg.Objects)
	pkgLabel := PKG_NAME_LABEL + "=" + pkg.Name
	args := []string{"apply", "--wait=true", "--timeout=2m", "-f", "-", "--record"}
	if prune {
		// TODO: delete objects within other namespaces that belong to the package as well
		args = append(args, "-l", pkgLabel, "--prune")
	}
	cmd := newKubectlCmd(ctx, m.kubeconfigFile)
	cmd.Stdin = reader
	err = cmd.Run(args...)
	if e := reader.Close(); e != nil && err == nil {
		err = e
	}
	if err == nil {
		var obj []*model.K8sObject
		if obj, err = m.objectState(ctx, pkg.Objects); err == nil {
			if err = m.awaitRollout(ctx, obj); err == nil {
				err = m.awaitCondition(ctx, obj)
			}
		}
	}
	return errors.Wrapf(err, "apply package %s", pkg.Name)
}

func (m *PackageManager) objectState(ctx context.Context, obj []*model.K8sObject) (state []*model.K8sObject, err error) {
	nsMap, nsOrder := groupByNamespace(obj)
	for _, ns := range nsOrder {
		ol := nsMap[ns]
		args := make([]string, len(ol))
		for i, o := range ol {
			args[i] = strings.ToLower(o.Kind) + "/" + o.Name
		}
		ol, e := m.kubectlGet(ctx, ns, args)
		if e == nil {
			state = append(state, ol...)
		} else if err == nil {
			err = e
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
	}
	return
}

func (m *PackageManager) awaitRollout(ctx context.Context, obj []*model.K8sObject) (err error) {
	obj = filter(obj, func(o *model.K8sObject) bool {
		return o.Kind == "Deployment" || o.Kind == "DaemonSet" || o.Kind == "StatefulSet"
	})
	for _, o := range obj {
		args := []string{"rollout", "status", "-w", "--timeout=2m"}
		if o.Namespace != "" {
			args = append(args, "-n", o.Namespace)
		}
		args = append(args, strings.ToLower(o.Kind)+"/"+o.Name)

		if err = newKubectlCmd(ctx, m.kubeconfigFile).Run(args...); err != nil {
			return
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}
	return
}

func (m *PackageManager) awaitCondition(ctx context.Context, obj []*model.K8sObject) (err error) {
	cmd := newKubectlCmd(ctx, m.kubeconfigFile)
	ctMap := map[string][]*model.K8sObject{}
	keys := []string{}
	for _, o := range obj {
		for _, ct := range o.ConditionTypes {
			ol := ctMap[ct]
			if ol == nil {
				keys = append(keys, ct)
			}
			ctMap[ct] = append(ol, o)
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(keys)))
	for _, ct := range keys {
		ol := ctMap[ct]
		logrus.Debugf("Waiting for %d components to become %s...", len(ol), ct)
		if err = kubectlWait(cmd, ol, "condition="+ct); err != nil {
			return
		}
	}
	return
}

func kubectlWait(cmd *kubectlCmd, obj []*model.K8sObject, forExpr string) (err error) {
	nsMap, nsOrder := groupByNamespace(obj)
	for _, ns := range nsOrder {
		if e := kubectlWaitNames(cmd, ns, names(nsMap[ns]), forExpr); e != nil {
			err = e
		}
		select {
		case <-cmd.ctx.Done():
			return cmd.ctx.Err()
		default:
		}
	}
	return
}

func kubectlWaitNames(cmd *kubectlCmd, ns string, names []string, forExpr string) (err error) {
	if len(names) == 0 {
		return
	}
	args := []string{"wait", "--for", forExpr, "--timeout=2m"}
	if ns != "" {
		args = append(args, "-n", ns)
	}
	if e := cmd.Run(append(args, names...)...); e != nil && err == nil {
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

func (m *PackageManager) Delete(ctx context.Context, namespace, pkgName string) (err error) {
	pkg, err := m.State(ctx, namespace, pkgName)
	if err != nil {
		return
	}
	if err = m.DeleteObjects(ctx, pkg.Objects); err != nil {
		// Workaround to exit successfully in case `kubectl wait` did not find an already deleted resource.
		// This should be solved within kubectl so that it does not exit with an error when waiting for deletion of a deleted resource.
		pkg, e := m.State(ctx, namespace, pkgName)
		if e == nil {
			err = errors.Errorf("leftover resources: %s", strings.Join(names(pkg.Objects), ", "))
		} else if IsNotFound(e) {
			err = nil
		}
	}
	return errors.Wrapf(err, "delete package %s", pkgName)
}

func (m *PackageManager) DeleteObjects(ctx context.Context, obj []*model.K8sObject) (err error) {
	defer m.clearResourceTypeCache()
	fqnMap := map[string]bool{}
	crds := filter(obj, func(o *model.K8sObject) bool { return o.Kind == "CustomResourceDefinition" })
	mapFqns(crds, fqnMap)
	crdMap := crdGvkMap(crds)
	crdRes := filter(obj, func(o *model.K8sObject) bool { return crdMap[o.Gvk()] })
	mapFqns(crdRes, fqnMap)
	namespaced := filter(obj, func(o *model.K8sObject) bool { return o.Namespace != "" && !fqnMap[o.ID()] })
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
			if e := m.deleteObjectNames(ctx, ns, names(nonContained)); e != nil && err == nil {
				err = e
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
		}
		for _, ns := range nsOrder {
			cmd := newKubectlCmd(ctx, m.kubeconfigFile)
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			if e := kubectlWaitNames(cmd, ns, names(nsMap[ns]), "delete"); e != nil && err == nil {
				// TODO: check if objects still exist to resolve error
				msg := e.Error()
				sout := stdout.String()
				serr := stderr.String()
				if sout != "" {
					msg += ", stdout: " + sout
				}
				if serr != "" {
					msg += ", stderr: " + serr
				}
				err = errors.New(msg)
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
		}
	}
	return
}

func (m *PackageManager) deleteObjectNames(ctx context.Context, ns string, names []string) (err error) {
	if len(names) == 0 {
		return
	}
	args := []string{"delete", "--wait=true", "--timeout=2m", "--cascade=true", "--ignore-not-found=true"}
	if ns != "" {
		args = append(args, "-n", ns)
	}
	return newKubectlCmd(ctx, m.kubeconfigFile).Run(append(args, names...)...)
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

func manifestReader(objects []*model.K8sObject) io.ReadCloser {
	reader, writer := io.Pipe()
	go func() {
		writer.CloseWithError(model.WriteManifest(objects, writer))
	}()
	return reader
}

type kubectlCmd struct {
	ctx            context.Context
	kubeconfigFile string
	Stdout         io.Writer
	Stderr         io.Writer
	Stdin          io.Reader
}

func newKubectlCmd(ctx context.Context, kubeconfigFile string) *kubectlCmd {
	return &kubectlCmd{
		ctx:            ctx,
		kubeconfigFile: kubeconfigFile,
		Stdout:         os.Stdout,
		Stderr:         os.Stderr,
	}
}

func (c *kubectlCmd) Run(args ...string) (err error) {
	if c.kubeconfigFile != "" {
		args = append([]string{"--kubeconfig", c.kubeconfigFile}, args...)
	}
	cmd := exec.CommandContext(c.ctx, "kubectl", args...)
	cmd.Stdout = c.Stdout
	cmd.Stderr = c.Stderr
	cmd.Stdin = c.Stdin
	logrus.Debugf("Running %+v", cmd.Args)
	return errors.Wrapf(cmd.Run(), "%+v", cmd.Args)
}
