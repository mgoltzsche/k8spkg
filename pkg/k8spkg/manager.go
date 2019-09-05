package k8spkg

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/mgoltzsche/k8spkg/pkg/model"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	// See https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/
	PKG_NAME_LABEL = "app.kubernetes.io/part-of"
	PKG_NS_LABEL   = "k8spkg.mgoltzsche.github.com/namespaces"
	defaultTimeout = time.Duration(2 * time.Minute)
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
	var buf bytes.Buffer
	c.Stderr = &buf
	args = append([]string{"get", "-o", "yaml"}, args...)
	if namespace != "" {
		args = append(args, "-n", namespace)
	}
	err = c.Run(args...)
	writer.Close()
	if e := <-errc; e != nil && err == nil {
		err = e
	}
	stderr := buf.String()
	if err != nil && len(stderr) > 0 {
		err = errors.Errorf("%s, stderr: %s", err, strings.ReplaceAll(stderr, "\n", "\n  "))
	}
	return
}

func getTimeout(ctx context.Context) string {
	t, ok := ctx.Deadline()
	if ok {
		return t.Sub(time.Now()).String()
	}
	return defaultTimeout.String()
}

func (m *PackageManager) Apply(ctx context.Context, pkg *K8sPackage, prune bool) (err error) {
	logrus.Infof("Applying package %s", pkg.Name)
	startTime := time.Now()
	reader := manifestReader(pkg.Objects)
	pkgLabel := PKG_NAME_LABEL + "=" + pkg.Name

	args := []string{"apply", "-o", "yaml", "--wait=true", "--timeout=" + getTimeout(ctx), "-f", "-", "--record"}
	if prune {
		// TODO: delete objects within other namespaces that belong to the package as well
		args = append(args, "-l", pkgLabel, "--prune")
	}
	pReader, pWriter := io.Pipe()
	objCh := make(chan []*model.K8sObject)
	errCh := make(chan error)
	go func() {
		obj, e := model.FromReader(pReader)
		pReader.CloseWithError(e)
		objCh <- obj
		errCh <- e
	}()
	cmd := newKubectlCmd(ctx, m.kubeconfigFile)
	cmd.Stdin = reader
	cmd.Stdout = pWriter
	err = cmd.Run(args...)
	e1 := reader.Close()
	e2 := pWriter.Close()
	obj := <-objCh
	e3 := <-errCh
	close(objCh)
	close(errCh)
	for _, e := range []error{e3, e1, e2} {
		if e != nil && err == nil {
			err = e
			break
		}
	}
	if err == nil {
		err = m.awaitChangesApplied(ctx, &K8sPackage{pkg.PackageInfo, obj}, startTime)
	}
	return errors.Wrapf(err, "apply package %s", pkg.Name)
}

func (m *PackageManager) awaitChangesApplied(ctx context.Context, pkg *K8sPackage, startTime time.Time) (err error) {
	obj := pkg.Objects
	uidMap := map[string]*model.K8sObject{}
	allUids := map[string]bool{}
	for _, o := range obj {
		uidMap[o.Uid] = o
	}
	ns := ""
	if len(pkg.Namespaces) > 0 {
		ns = pkg.Namespaces[0]
	}
	obj = nil
	if pkg, err = m.State(ctx, ns, pkg.Name); err != nil {
		return
	}
	for _, o := range pkg.Objects {
		allUids[o.Uid] = true
		if uidMap[o.Uid] != nil {
			obj = append(obj, o)
		}
	}

	errCh := make(chan error)
	go func() {
		e := m.awaitRollout(ctx, obj)
		if e == nil {
			e = m.awaitCondition(ctx, obj)
		}
		errCh <- e
	}()

	ctx, cancel := context.WithCancel(ctx)
	evtCh, evtErrCh := EventChannel(ctx, m.kubeconfigFile)
	var evt *Event
	done := 0
	for {
		select {
		case evt = <-evtCh:
			if allUids[evt.InvolvedObject.Uid] {
				logEvent(evt, startTime)
			}
		case evtErr := <-evtErrCh:
			if done == 0 {
				select {
				case <-ctx.Done():
				default:
					logrus.Warnf("error while streaming events: %s", evtErr)
				}
			}
			done++
		case err = <-errCh:
			cancel()
			done++
		}
		if done == 2 {
			break
		}
	}
	close(evtCh)
	close(errCh)
	close(evtErrCh)
	if err != nil {
		ctx, _ = context.WithTimeout(context.Background(), defaultTimeout)
		descr, e := m.describeFailureCause(ctx, obj)
		if e != nil {
			descr = "  find error cause: " + e.Error()
		}
		if descr != "" {
			err = errors.Errorf("%s\n\n  STATUS REPORT:\n\n%s", err, descr)
		}
	}
	return
}

func logEvent(evt *Event, startTime time.Time) {
	kind := strings.ToLower(evt.InvolvedObject.Kind)
	name := evt.InvolvedObject.Name
	ns := evt.InvolvedObject.Namespace
	log := logrus.WithField("id", kind+"/"+name).WithField("ns", ns)
	switch evt.Type {
	case "Normal":
		log.Infof("%s: %s (%dx)", evt.Reason, evt.Message, evt.Count)
	case "Warning":
		log.Warnf("%s: %s (%dx)", evt.Reason, evt.Message, evt.Count)
	default:
		log.Errorf("%s %s: %s (%dx)", evt.Type, evt.Reason, evt.Message, evt.Count)
	}
}

func (m *PackageManager) objectState(ctx context.Context, obj []*model.K8sObject) (state []*model.K8sObject, err error) {
	nsMap, nsOrder := groupByNamespace(obj)
	for _, ns := range nsOrder {
		ol := nsMap[ns]
		args := make([]string, len(ol)+1)
		args[0] = "--ignore-not-found"
		for i, o := range ol {
			args[i+1] = strings.ToLower(o.Kind) + "/" + o.Name
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
		args := []string{"rollout", "status", "-w", "--timeout=" + getTimeout(ctx)}
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

func (m *PackageManager) describeFailureCause(ctx context.Context, obj []*model.K8sObject) (msg string, err error) {
	if obj, err = m.objectState(ctx, obj); err != nil {
		return
	}
	failedObj := []*model.K8sObject{}
	condMsgs := []string{}
	for _, o := range obj {
		var failedConds []string
		reasonMap := map[string]bool{}
		reasons := []string{}
		for _, cond := range o.Conditions {
			if !cond.Status {
				failedConds = append(failedConds, cond.Type)
				failedObj = append(failedObj, o)
				reason := cond.Reason
				if reason == "" {
					reason = "not " + cond.Type
				}
				if cond.Message != "" {
					reasonMap[reason] = false
					reason += ": " + cond.Message
				}
				if !reasonMap[reason] {
					reasonMap[reason] = true
					reasons = append(reasons, reason)
				}
			}
		}
		if len(failedConds) > 0 {
			uniqReasons := make([]string, 0, len(reasons))
			for _, reason := range reasons {
				if reasonMap[reason] {
					uniqReasons = append(uniqReasons, reason)
				}
			}
			failedCondStr := strings.Join(failedConds, ", ")
			failureCauses := strings.Join(uniqReasons, "\n  - ")
			condMsgs = append(condMsgs, fmt.Sprintf("  %s has not met status conditions %s:\n  - %s", o.ID(), failedCondStr, failureCauses))
		}
	}
	sort.Strings(condMsgs)
	msg = strings.Join(condMsgs, "\n\n")
	return
}

func (m *PackageManager) awaitCondition(ctx context.Context, obj []*model.K8sObject) (err error) {
	cmd := newKubectlCmd(ctx, m.kubeconfigFile)
	ctMap := map[string][]*model.K8sObject{}
	types := []string{}
	for _, o := range obj {
		for _, c := range o.Conditions {
			ol := ctMap[c.Type]
			if ol == nil {
				types = append(types, c.Type)
			}
			ctMap[c.Type] = append(ol, o)
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(types)))
	for _, ct := range types {
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
	args := []string{"wait", "--for", forExpr, "--timeout=" + getTimeout(cmd.ctx)}
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
	err = m.DeleteObjects(ctx, pkg.Objects)
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
	orphanObj := []*model.K8sObject{}

	deletionOrder := [][]*model.K8sObject{
		crdRes,
		namespaced,
		other,
		crds,
	}

	var waitErr error
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
			ol := nsMap[ns]
			if e := kubectlWaitNames(cmd, ns, names(ol), "delete"); e != nil {
				orphanObj = append(orphanObj, ol...)
				if waitErr == nil {
					msg := e.Error()
					sout := stdout.String()
					serr := stderr.String()
					if sout != "" {
						msg += ", stdout: " + sout
					}
					if serr != "" {
						msg += ", stderr: " + serr
					}
					waitErr = errors.New(msg)
				}
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
		}
	}
	if err == nil && waitErr != nil {
		// Workaround to exit successfully in case `kubectl wait` did not find an already deleted resource.
		// This should be solved within kubectl so that it does not exit with an error when waiting for deletion of a deleted resource.
		orphanObj, _ = m.objectState(ctx, orphanObj)
		if len(orphanObj) != 0 {
			err = errors.Errorf("%d/%d objects could not be deleted: %+v", len(obj)-len(orphanObj), len(obj), names(orphanObj))
		}
	}
	return
}

func (m *PackageManager) deleteObjectNames(ctx context.Context, ns string, names []string) (err error) {
	if len(names) == 0 {
		return
	}
	args := []string{"delete", "--wait=true", "--timeout=" + getTimeout(ctx), "--cascade=true", "--ignore-not-found=true"}
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
