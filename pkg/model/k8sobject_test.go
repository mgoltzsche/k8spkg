package model

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestK8sObject(t *testing.T) {
	manifest := `apiVersion: certmanager.k8s.io/v1alpha1
kind: Issuer
metadata:
    labels:
        app.kubernetes.io/part-of: mypkg
    name: ca-issuer
    namespace: cert-manager
spec:
    someattr: avalue
`
	obj := map[string]interface{}{}
	err := yaml.Unmarshal([]byte(manifest), obj)
	require.NoError(t, err)
	o := K8sObject(obj)
	assert.Equal(t, "certmanager.k8s.io/v1alpha1", o.ApiVersion(), "apiVersion")
	assert.Equal(t, "Issuer", o.Kind(), "kind")
	assert.Equal(t, "ca-issuer", o.Metadata().Name, "name")
	assert.Equal(t, "cert-manager", o.Metadata().Namespace, "namespace")

	var buf bytes.Buffer
	err = o.WriteYaml(&buf)
	require.NoError(t, err)
	assert.Equal(t, "---\n"+manifest, buf.String(), "yamlIn->obj->yamlOut == yamlIn")
}

func TestK8sObjectGetString(t *testing.T) {
	manifest := `apiVersion: certmanager.k8s.io/v1alpha1
kind: Issuer
metadata:
    labels:
        app.kubernetes.io/part-of: mypkg
    name: ca-issuer
    namespace: cert-manager
spec:
    someattr: avalue
`
	obj := map[string]interface{}{}
	err := yaml.Unmarshal([]byte(manifest), obj)
	require.NoError(t, err)
	o := K8sObject(obj)
	var buf bytes.Buffer
	err = o.WriteYaml(&buf)
	require.NoError(t, err)
	assert.Equal(t, "avalue", o.GetString("spec.someattr"))
}

func TestK8sObjectFromReader(t *testing.T) {
	f, err := os.Open("test/k8sobjectlist.yaml")
	require.NoError(t, err)
	defer f.Close()
	ol, err := K8sObjectsFromReader(f)
	require.NoError(t, err)
	names := []string{}
	for _, o := range ol {
		names = append(names, o.Metadata().Name)
	}
	expectedNames := []string{"certificates.certmanager.k8s.io", "somedeployment", "myapiservice", "mydeployment", "onemorecert"}
	assert.Equal(t, expectedNames, names, "flattened object names")
}
