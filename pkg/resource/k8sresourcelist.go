package resource

import (
	"io"

	"gopkg.in/yaml.v2"
)

type K8sResourceList []*K8sResource

type K8sResourceGroup struct {
	Key       string
	Resources K8sResourceList
}

func FromReader(f io.Reader) (l K8sResourceList, err error) {
	dec := yaml.NewDecoder(f)
	obj := []*K8sResource{}
	o := map[string]interface{}{}
	for ; err == nil; err = dec.Decode(o) {
		if len(o) > 0 {
			if err = appendFlattened(o, &obj); err != nil {
				return
			}
			o = map[string]interface{}{}
		}
	}
	if err == io.EOF {
		err = nil
	}
	return obj, err
}

func (l K8sResourceList) WriteYaml(writer io.Writer) (err error) {
	for _, o := range l {
		if err = o.WriteYaml(writer); err != nil {
			return
		}
	}
	return
}

func (l K8sResourceList) Filter(filter func(*K8sResource) bool) (filtered K8sResourceList) {
	for _, o := range l {
		if filter(o) {
			filtered = append(filtered, o)
		}
	}
	return
}

func (l K8sResourceList) GroupByNamespace() (groups []*K8sResourceGroup) {
	return l.groupBy(func(o *K8sResource) string { return o.Namespace })
}

func (l K8sResourceList) GroupByKind() (groups []*K8sResourceGroup) {
	return l.groupBy(func(o *K8sResource) string { return o.Kind })
}

func (l K8sResourceList) groupBy(keyFn func(*K8sResource) string) (groups []*K8sResourceGroup) {
	grouped := map[string]*K8sResourceGroup{}
	for _, o := range l {
		key := keyFn(o)
		g := grouped[key]
		if g == nil {
			g = &K8sResourceGroup{key, []*K8sResource{o}}
			groups = append(groups, g)
			grouped[key] = g
		} else {
			g.Resources = append(g.Resources, o)
		}
	}
	return
}
