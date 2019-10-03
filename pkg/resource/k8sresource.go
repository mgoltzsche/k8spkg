package resource

import (
	"fmt"
	"io"
	"strings"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

type rawK8sResource map[string]interface{}

type K8sResource struct {
	raw        rawK8sResource
	APIVersion string
	Kind       string
	Namespace  string
	Name       string
	Uid        string
	Conditions []*K8sResourceCondition
}

type K8sResourceCondition struct {
	Type    string
	Status  bool
	Reason  string
	Message string
}

func FromMap(o map[string]interface{}) *K8sResource {
	meta := asMap(o["metadata"])
	rawConditions := asList(lookup(o, "status.conditions"))
	conditions := make([]*K8sResourceCondition, 0, len(rawConditions))
	for _, entry := range rawConditions {
		rawCondition := asMap(entry)
		ct := asString(rawCondition["type"])
		if ct != "" {
			conditions = append(conditions, &K8sResourceCondition{
				strings.ToLower(ct),
				strings.ToLower(asString(rawCondition["status"])) == "true",
				asString(rawCondition["reason"]),
				asString(rawCondition["message"]),
			})
		}
	}
	return &K8sResource{
		raw:        o,
		APIVersion: asString(o["apiVersion"]),
		Kind:       asString(o["kind"]),
		Namespace:  asString(meta["namespace"]),
		Name:       asString(meta["name"]),
		Uid:        asString(meta["uid"]),
		Conditions: conditions,
	}
}

func (o *K8sResource) Validate() (err error) {
	if o.APIVersion == "" || o.Kind == "" || o.Name == "" {
		err = errors.Errorf("invalid API object: apiVersion, kind or name are not set: %+v", o.raw)
	}
	return
}

func appendFlattened(o rawK8sResource, flattened *[]*K8sResource) (err error) {
	if o["kind"] != "List" {
		entry := FromMap(o)
		err = entry.Validate()
		*flattened = append(*flattened, entry)
		return
	}
	ol, err := items(o)
	if err == nil {
		for _, o := range ol {
			if err = appendFlattened(o, flattened); err != nil {
				return
			}
		}
	}
	return
}

// Returns 'items' slice
func items(o rawK8sResource) (items []rawK8sResource, err error) {
	rawItems := asList(o["items"])
	if rawItems == nil {
		return nil, errors.New("object of kind List does not declare items")
	}
	items = make([]rawK8sResource, len(rawItems))
	for i, item := range rawItems {
		items[i] = rawK8sResource(asMap(item))
	}
	return
}

func (o *K8sResource) ID() string {
	return o.Namespace + "/" + o.Gvk() + "/" + o.Name
}

func (o *K8sResource) Gvk() string {
	return o.APIVersion + "/" + o.Kind
}

func (o *K8sResource) Labels() (l map[string]string) {
	l = map[string]string{}
	for k, v := range asMap(asMap(o.raw["metadata"])["labels"]) {
		l[k] = asString(v)
	}
	return
}

func (o *K8sResource) CrdGvk() string {
	group := o.getString("spec.group")
	version := o.getString("spec.version")
	kind := o.getString("spec.names.kind")
	return group + "/" + version + "/" + kind
}

type OwnerReference struct {
	APIVersion string
	Kind       string
	Name       string
	UID        string
}

func (o *K8sResource) OwnerReferences() (r []*OwnerReference) {
	for _, ref := range asList(lookup(o.raw, "metadata.ownerReferences")) {
		r = append(r, &OwnerReference{
			APIVersion: asString(asMap(ref)["apiVersion"]),
			Kind:       asString(asMap(ref)["kind"]),
			Name:       asString(asMap(ref)["name"]),
			UID:        asString(asMap(ref)["uuid"]),
		})
	}
	return
}

func (o *K8sResource) WriteYaml(writer io.Writer) (err error) {
	if _, err = writer.Write([]byte("---\n")); err == nil {
		err = yaml.NewEncoder(writer).Encode(o.raw)
	}
	return errors.Wrapf(err, "encode k8sobject %s/%s to yaml", o.Kind, o.Name)
}

func lookup(o map[string]interface{}, path string) (r interface{}) {
	var m map[string]interface{} = o
	r = m
	if path != "" {
		segments := strings.Split(path, ".")
		for _, property := range segments[:len(segments)-1] {
			m = asMap(m[property])
		}
		r = m[segments[len(segments)-1]]
	}
	return
}

func (o *K8sResource) getString(path string) string {
	return asString(lookup(o.raw, path))
}

func asList(o interface{}) []interface{} {
	if l, ok := o.([]interface{}); ok {
		return l
	}
	return nil
}

func asString(o interface{}) (s string) {
	if o != nil {
		s = fmt.Sprintf("%s", o)
	}
	return
}

func asMap(o interface{}) (m map[string]interface{}) {
	if o != nil {
		if mc, ok := o.(map[string]interface{}); ok {
			return mc
		} else if mc, ok := o.(map[interface{}]interface{}); ok {
			m = map[string]interface{}{}
			for k, v := range mc {
				m[k.(string)] = v
			}
			return
		}
	}
	return map[string]interface{}{}
}
