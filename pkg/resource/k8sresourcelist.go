package resource

import (
	"io"

	"gopkg.in/yaml.v2"
)

type K8sResourceList []*K8sResource

func FromYaml(reader io.Reader) (l K8sResourceList, err error) {
	dec := yaml.NewDecoder(reader)
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
