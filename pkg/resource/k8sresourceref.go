package resource

import (
	"fmt"
	"strings"
)

type K8sResourceRef interface {
	APIVersion() string
	Kind() string
	Name() string
	Namespace() string
	QualifiedKind() string
	ID() string
}

type k8sResourceRef struct {
	apiVersion string
	kind       string
	name       string
	namespace  string
}

func ResourceRef(apiVersion, kind, namespace, name string) *k8sResourceRef {
	return &k8sResourceRef{
		apiVersion: apiVersion,
		kind:       kind,
		name:       name,
		namespace:  namespace,
	}
}

func (o *k8sResourceRef) APIVersion() string {
	return o.apiVersion
}
func (o *k8sResourceRef) Kind() string {
	return o.kind
}
func (o *k8sResourceRef) Name() string {
	return o.name
}
func (o *k8sResourceRef) Namespace() string {
	return o.namespace
}
func (o *k8sResourceRef) ID() string {
	return fmt.Sprintf("%s:%s:%s", o.QualifiedKind(), o.namespace, o.name)
}
func (o *k8sResourceRef) String() string {
	return o.ID()
}
func (o *k8sResourceRef) QualifiedKind() (kind string) {
	apiGroup := ""
	if gv := strings.SplitN(o.apiVersion, "/", 2); len(gv) == 2 {
		apiGroup = gv[0]
	}
	kind = strings.ToLower(o.kind)
	if apiGroup != "" {
		kind += "." + apiGroup
	}
	return
}

type K8sResourceRefList []K8sResourceRef

type K8sResourceGroup struct {
	Key       string
	Resources K8sResourceRefList
}

func (l K8sResourceRefList) GroupByNamespace() (groups []*K8sResourceGroup) {
	return l.groupBy(func(o K8sResourceRef) string { return o.Namespace() })
}

func (l K8sResourceRefList) GroupByKind() (groups []*K8sResourceGroup) {
	return l.groupBy(func(o K8sResourceRef) string { return o.Kind() })
}

func (l K8sResourceRefList) groupBy(keyFn func(K8sResourceRef) string) (groups []*K8sResourceGroup) {
	grouped := map[string]*K8sResourceGroup{}
	for _, o := range l {
		key := keyFn(o)
		g := grouped[key]
		if g == nil {
			g = &K8sResourceGroup{key, []K8sResourceRef{o}}
			groups = append(groups, g)
			grouped[key] = g
		} else {
			g.Resources = append(g.Resources, o)
		}
	}
	return
}

func (l K8sResourceRefList) Names() (names []string) {
	names = make([]string, len(l))
	for i, o := range l {
		names[i] = o.QualifiedKind() + "/" + o.Name()
	}
	return
}

func (l K8sResourceRefList) Filter(filter func(K8sResourceRef) bool) (filtered K8sResourceRefList) {
	for _, o := range l {
		if filter(o) {
			filtered = append(filtered, o)
		}
	}
	return
}
