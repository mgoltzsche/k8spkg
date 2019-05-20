package main

import (
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

type K8sObjects []K8sObject

func (l K8sObjects) toYaml(writer io.Writer) (err error) {
	for _, o := range l {
		if _, err = writer.Write([]byte("---\n")); err != nil {
			return
		}
		if err = yaml.NewEncoder(writer).Encode(o); err != nil {
			return
		}
	}
	return
}

type K8sObject map[string]interface{}

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
	m := o["metadata"]
	var name, labels interface{}
	if mc, ok := m.(map[string]interface{}); ok && m != nil {
		name = mc["name"]
		labels = mc["labels"]
	} else if mc, ok := m.(map[interface{}]interface{}); ok && m != nil {
		name = mc["name"]
		labels = mc["labels"]
	}
	if name != nil {
		meta.Name = name.(string)
	}
	meta.Labels = map[string]string{}
	if labels != nil {
		for k, v := range labels.(map[string]interface{}) {
			if v == nil {
				meta.Labels[k] = ""
			} else {
				meta.Labels[k] = v.(string)
			}
		}
	}
	return
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

// see https://kubernetes.io/docs/reference/federation/v1/definitions/#_v1_objectmeta
type K8sObjectMeta struct {
	Name   string            `yaml:"name,omitempty"`
	Labels map[string]string `yaml:"labels,omitempty"`
}
