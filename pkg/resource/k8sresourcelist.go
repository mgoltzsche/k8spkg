package resource

import (
	"io"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
)

type K8sResourceList []*K8sResource

func FromReader(reader io.Reader) (l K8sResourceList, err error) {
	obj := []*K8sResource{}
	o := map[string]interface{}{}
	dec := yaml.NewYAMLOrJSONDecoder(reader, 1024)
	for ; err == nil; err = dec.Decode(&o) {
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

func (l K8sResourceList) Refs() K8sResourceRefList {
	r := make([]K8sResourceRef, len(l))
	for i, o := range l {
		r[i] = o
	}
	return r
}

func (l K8sResourceList) WriteYaml(writer io.Writer) (err error) {
	for _, o := range l {
		if err = o.WriteYaml(writer); err != nil {
			return
		}
	}
	return
}

func (l K8sResourceList) YamlReader() io.ReadCloser {
	reader, writer := io.Pipe()
	go func() {
		writer.CloseWithError(l.WriteYaml(writer))
	}()
	return reader
}

func appendFlattened(o rawK8sResource, flattened *[]*K8sResource) (err error) {
	obj := unstructured.Unstructured{o}
	if obj.IsList() {
		return obj.EachListItem(func(o runtime.Object) error {
			return appendFlattened(o.(*unstructured.Unstructured).Object, flattened)
		})
	}
	entry := FromMap(o)
	err = entry.Validate()
	*flattened = append(*flattened, entry)
	return
}
