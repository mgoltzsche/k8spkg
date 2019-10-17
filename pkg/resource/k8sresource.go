package resource

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

type rawK8sResource map[string]interface{}

type K8sResource struct {
	K8sResourceRef
	raw        rawK8sResource
	uid        string
	conditions []*K8sResourceCondition
}

func Resource(name K8sResourceRef, attrs map[string]interface{}) *K8sResource {
	apiVersion := name.APIVersion()
	if name.APIGroup() != "" {
		apiVersion = name.APIGroup() + "/" + apiVersion
	}
	attrs["apiVersion"] = apiVersion
	attrs["kind"] = name.Kind()
	meta := map[string]interface{}{}
	meta["name"] = name.Name()
	meta["namespace"] = name.Namespace()
	attrs["metadata"] = meta
	return &K8sResource{name, attrs, "", nil}
}

type ResourceEvent struct {
	Resource *K8sResource
	Error    error
}

// TODO: test
func FromJsonStream(reader io.Reader) <-chan ResourceEvent {
	ch := make(chan ResourceEvent)
	go func() {
		var err error
		dec := json.NewDecoder(reader)
		o := map[string]interface{}{}
		l := make([]*K8sResource, 0, 1)
		for ; err == nil; err = dec.Decode(&o) {
			if len(o) > 0 {
				l = l[:0]
				if err = appendFlattened(o, &l); err != nil {
					break
				}
				for _, lo := range l {
					ch <- ResourceEvent{lo, nil}
				}
				o = map[string]interface{}{}
			}
		}
		if err != nil && err != io.EOF {
			ch <- ResourceEvent{nil, err}
		}
		close(ch)
	}()
	return ch
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
	apiVersion := asString(o["apiVersion"])
	apiGroup := ""
	if gv := strings.SplitN(apiVersion, "/", 2); len(gv) == 2 {
		apiGroup = gv[0]
		apiVersion = gv[1]
	}
	return &K8sResource{
		raw: o,
		K8sResourceRef: &k8sResourceRef{
			apiVersion: apiVersion,
			apiGroup:   apiGroup,
			kind:       asString(o["kind"]),
			name:       asString(meta["name"]),
			namespace:  asString(meta["namespace"]),
		},
		uid:        asString(meta["uid"]),
		conditions: conditions,
	}
}

func (o *K8sResource) Validate() (err error) {
	if o.APIVersion() == "" || o.Kind() == "" || o.Name() == "" {
		err = errors.Errorf("invalid API object: apiVersion, kind or name are not set: %+v", o.raw)
	}
	return
}

func (o *K8sResource) Conditions() []*K8sResourceCondition {
	return o.conditions
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

func (o *K8sResource) Spec() (l map[string]interface{}) {
	return asMap(o.raw["spec"])
}

func (o *K8sResource) SetSpec(spec map[string]interface{}) {
	o.raw["spec"] = spec
}

func (o *K8sResource) Labels() (l map[string]string) {
	l = map[string]string{}
	for k, v := range asMap(asMap(o.raw["metadata"])["labels"]) {
		l[k] = asString(v)
	}
	return
}

func (o *K8sResource) CrdQualifiedKind() string {
	group := o.getString("spec.group")
	kind := strings.ToLower(o.getString("spec.names.kind"))
	if group != "" {
		kind += "." + group
	}
	return kind
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
	return errors.Wrapf(err, "encode k8sobject %s to yaml", o.ID())
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
