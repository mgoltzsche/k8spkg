package main

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestK8sObject(t *testing.T) {
	manifest := `
apiVersion: certmanager.k8s.io/v1alpha1
kind: ClusterIssuer
metadata:
  name: ca-issuer
attr: avalue`
	obj := map[string]interface{}{}
	if err := yaml.Unmarshal([]byte(manifest), obj); err != nil {
		t.Error(err)
		t.FailNow()
	}
	o := K8sObject(obj)
	if o.ApiVersion() != "certmanager.k8s.io/v1alpha1" {
		t.Errorf("unexpected apiVersion %q", o.ApiVersion())
	}
	if o.Kind() != "ClusterIssuer" {
		t.Errorf("unexpected kind %q", o.Kind())
	}
	if o.Metadata().Name != "ca-issuer" {
		t.Errorf("unexpected metadata.name %q", o.Metadata().Name)
	}
	if o["attr"] != "avalue" {
		t.Errorf("unexpected attr value %q", o["attr"])
	}
}
