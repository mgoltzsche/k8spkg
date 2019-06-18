package model

import (
	"fmt"
	"io"
	"strings"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

const (
	// See https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/
	PKG_LABEL = "app.kubernetes.io/part-of"
)

type K8sPackage struct {
	Name    string
	Objects []K8sObject
}

func NewK8sPackage(pkgName string, objects []K8sObject) *K8sPackage {
	if pkgName == "" {
		panic("empty pkgName provided")
	}
	// Sets k8s's part-of and managed-by labels.
	// See https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/
	for _, o := range objects {
		m := o.Metadata()
		m.Labels["app.kubernetes.io/managed-by"] = "k8spkg"
		m.Labels[PKG_LABEL] = pkgName
		o.SetMetadata(m)
	}
	return &K8sPackage{pkgName, objects}
}

func (pkg *K8sPackage) ToYaml(writer io.Writer) (err error) {
	for _, o := range pkg.Objects {
		if err = o.WriteYaml(writer); err != nil {
			return
		}
	}
	return
}

type K8sObject map[string]interface{}

func K8sObjectsFromReader(f io.Reader) (obj []K8sObject, err error) {
	dec := yaml.NewDecoder(f)
	o := map[string]interface{}{}
	for ; err == nil; err = dec.Decode(o) {
		if len(o) > 0 {
			if err = appendFlattened(K8sObject(o), &obj); err != nil {
				return
			}
			o = map[string]interface{}{}
		}
	}
	if err == io.EOF {
		err = nil
	}
	return
}

func appendFlattened(o K8sObject, flattened *[]K8sObject) (err error) {
	if o.Kind() == "List" {
		var ol []K8sObject
		if ol, err = items(o); err == nil {
			for _, o := range ol {
				if err = appendFlattened(o, flattened); err != nil {
					return
				}
			}
		}
	} else {
		*flattened = append(*flattened, o)
	}
	return
}

// Returns child objects in case kind=List
func items(o K8sObject) (items []K8sObject, err error) {
	if node, ok := o["items"].([]interface{}); !ok || node != nil {
		if !ok {
			return nil, errors.New("items property is not a slice")
		}
		items = make([]K8sObject, len(node))
		for i, o := range node {
			om, ok := o.(map[string]interface{})
			if !ok {
				oms, oks := o.(map[interface{}]interface{})
				if oks {
					om = map[string]interface{}{}
					for k, v := range oms {
						ks, ok := k.(string)
						if !ok {
							return nil, errors.Errorf("item key %q is not an string", k)
						}
						om[ks] = v
					}
				}
			}
			if om == nil {
				return nil, errors.Errorf("item is not an object but %T", o)
			}
			items[i] = K8sObject(om)
		}
	}
	return
}

func (o K8sObject) ApiVersion() string {
	if o["apiVersion"] == nil {
		return ""
	} else {
		return o["apiVersion"].(string)
	}
}

func (o K8sObject) Kind() string {
	if o["kind"] == nil {
		return ""
	} else {
		return o["kind"].(string)
	}
}

func (o K8sObject) Metadata() (meta K8sObjectMeta) {
	m := asMap(o["metadata"])
	meta.Namespace = asString(m["namespace"])
	meta.Name = asString(m["name"])
	meta.Labels = map[string]string{}
	for k, v := range asMap(m["labels"]) {
		meta.Labels[k] = asString(v)
	}
	return
}

func (o K8sObject) GetString(path string) string {
	lastDot := strings.LastIndex(path, ".")
	var m map[string]interface{} = o
	if lastDot >= 0 {
		m = lookupMap(m, path[:lastDot])
		path = path[lastDot+1:]
	}
	return asString(m[path])
}

func lookupMap(m map[string]interface{}, path string) (r map[string]interface{}) {
	r = m
	if path != "" {
		for _, property := range strings.Split(path, ".") {
			r = asMap(r[property])
		}
	}
	return
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

func (o K8sObject) SetMetadata(meta K8sObjectMeta) {
	m := o["metadata"]
	if m == nil {
		m = map[string]interface{}{}
		o["metadata"] = m
	}
	b, err := yaml.Marshal(&meta)
	if err != nil {
		panic(err)
	}
	if err = yaml.Unmarshal(b, m); err != nil {
		panic(err)
	}
}

func (o K8sObject) String() string {
	return fmt.Sprintf("%s/%s", o.Kind(), o.Metadata().Name)
}

func (o K8sObject) WriteYaml(writer io.Writer) (err error) {
	if _, err = writer.Write([]byte("---\n")); err == nil {
		err = yaml.NewEncoder(writer).Encode(o)
	}
	return errors.Wrapf(err, "encode k8sobject %q to yaml", o.Metadata().Name)
}

// see https://kubernetes.io/docs/reference/federation/v1/definitions/#_v1_objectmeta
type K8sObjectMeta struct {
	Name      string            `yaml:"name,omitempty"`
	Namespace string            `yaml:"namespace,omitempty"`
	Labels    map[string]string `yaml:"labels,omitempty"`
}
