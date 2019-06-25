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

type K8sPackage []*K8sObject

func (pkg K8sPackage) WriteYaml(writer io.Writer) (err error) {
	for _, o := range pkg {
		if err = o.WriteYaml(writer); err != nil {
			return
		}
	}
	return
}

type K8sObject struct {
	rawK8sObject
	APIVersion string
	Namespace  string
	Kind       string
	Name       string
}

type rawK8sObject map[string]interface{}

func NewK8sObject(o map[string]interface{}) *K8sObject {
	m := asMap(o["metadata"])
	return &K8sObject{
		rawK8sObject: rawK8sObject(o),
		APIVersion:   asString(o["apiVersion"]),
		Kind:         asString(o["kind"]),
		Namespace:    asString(m["namespace"]),
		Name:         asString(m["name"]),
	}
}

//type RawK8sObject map[string]interface{}

func K8sObjectsFromReader(f io.Reader) (obj []*K8sObject, err error) {
	dec := yaml.NewDecoder(f)
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
	return
}

func appendFlattened(o rawK8sObject, flattened *[]*K8sObject) (err error) {
	if o["kind"] != "List" {
		*flattened = append(*flattened, NewK8sObject(o))
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

// Returns child objects in case kind=List
func items(o rawK8sObject) (items []rawK8sObject, err error) {
	rawItems := asList(o["items"])
	if rawItems == nil {
		return nil, errors.New("object of kind List does not declare items")
	}
	items = make([]rawK8sObject, len(rawItems))
	for i, item := range rawItems {
		items[i] = rawK8sObject(asMap(item))
	}
	return
}

func (o *K8sObject) ID() string {
	return o.Namespace + "/" + o.Gvk() + "/" + o.Name
}

func (o *K8sObject) Gvk() string {
	return o.APIVersion + "/" + o.Kind
}

func (o *K8sObject) Labels() (l map[string]string) {
	l = map[string]string{}
	for k, v := range asMap(asMap(o.rawK8sObject["metadata"])["labels"]) {
		l[k] = asString(v)
	}
	return
}

func (o *K8sObject) CrdGvk() string {
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

func (o *K8sObject) OwnerReferences() (r []*OwnerReference) {
	for _, ref := range asList(lookup(o.rawK8sObject, "metadata.ownerReferences")) {
		r = append(r, &OwnerReference{
			APIVersion: asString(asMap(ref)["apiVersion"]),
			Kind:       asString(asMap(ref)["kind"]),
			Name:       asString(asMap(ref)["name"]),
			UID:        asString(asMap(ref)["uuid"]),
		})
	}
	return
}

func (o *K8sObject) WriteYaml(writer io.Writer) (err error) {
	if _, err = writer.Write([]byte("---\n")); err == nil {
		err = yaml.NewEncoder(writer).Encode(o.rawK8sObject)
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

func (o *K8sObject) getString(path string) string {
	return asString(lookup(o.rawK8sObject, path))
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
