package resource

import (
	"encoding/json"
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
	conditions []*K8sResourceCondition
}

func Resource(name K8sResourceRef, attrs map[string]interface{}) *K8sResource {
	attrs["apiVersion"] = name.APIVersion()
	attrs["kind"] = name.Kind()
	meta := map[string]interface{}{}
	meta["name"] = name.Name()
	meta["namespace"] = name.Namespace()
	attrs["metadata"] = meta
	return &K8sResource{name, attrs, nil}
}

type ResourceEvent struct {
	Resource *K8sResource
	Error    error
}

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
		rawCondition, ok := entry.(map[string]interface{})
		if !ok {
			continue
		}
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
	return &K8sResource{
		raw: o,
		K8sResourceRef: &k8sResourceRef{
			apiVersion: obj.GetAPIVersion(),
			kind:       obj.GetKind(),
			name:       obj.GetName(),
			namespace:  obj.GetNamespace(),
		},
		conditions: conditions,
	}
}

func (o *K8sResource) Validate() (err error) {
	if o.APIVersion() == "" || o.Kind() == "" || o.Name() == "" {
		err = errors.Errorf("invalid resource: apiVersion, kind or name are not set: %+v", o.raw)
	}
	return
}

func (o *K8sResource) Raw() (r map[string]interface{}) {
	return o.raw
}

func (o *K8sResource) Labels() (l map[string]string) {
	l, _, _ = unstructured.NestedStringMap(o.raw, "metadata", "labels")
	return
}

func (o *K8sResource) Conditions() []*K8sResourceCondition {
	return o.conditions
}

func (o *K8sResource) WriteYaml(writer io.Writer) (err error) {
	if _, err = writer.Write([]byte("---\n")); err == nil {
		err = yaml.NewEncoder(writer).Encode(o.raw)
	}
	return errors.Wrapf(err, "encode resource %s to yaml", o.ID())
}
