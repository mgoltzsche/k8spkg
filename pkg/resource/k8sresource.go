package resource

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
	obj := unstructured.Unstructured{o}
	rawConditions, _, _ := unstructured.NestedSlice(o, "status", "conditions")
	conditions := make([]*K8sResourceCondition, 0, len(rawConditions))
	for _, entry := range rawConditions {
		rawCondition := asMap(entry)
		if ct, _, _ := unstructured.NestedString(rawCondition, "type"); ct != "" {
			status, _, _ := unstructured.NestedString(rawCondition, "status")
			reason, _, _ := unstructured.NestedString(rawCondition, "reason")
			message, _, _ := unstructured.NestedString(rawCondition, "message")
			conditions = append(conditions, &K8sResourceCondition{
				strings.ToLower(ct),
				strings.ToLower(status) == "true",
				reason,
				message,
			})
		}
	}
	apiVersion := obj.GetAPIVersion()
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
			kind:       obj.GetKind(),
			name:       obj.GetName(),
			namespace:  obj.GetNamespace(),
		},
		uid:        string(obj.GetUID()),
		conditions: conditions,
	}
}

func (o *K8sResource) Validate() (err error) {
	if o.APIVersion() == "" || o.Kind() == "" || o.Name() == "" {
		err = errors.Errorf("invalid API object: apiVersion, kind or name are not set: %+v", o.raw)
	}
	return
}

// MatchLabels returns the labels on which pods are matched (in case of Deployment or DaemonSet)
func (o *K8sResource) SelectorMatchLabels() []string {
	m := asMap(lookup(o.raw, "spec.selector.matchLabels"))
	labels := make([]string, 0, len(m))
	for k, v := range m {
		labels = append(labels, fmt.Sprintf("%s=%s", k, v))
	}
	return labels
}

func (o *K8sResource) Raw() (r map[string]interface{}) {
	return o.raw
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
