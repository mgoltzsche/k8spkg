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
    ownerReferences:
      - apiVersion: apps/v1
        kind: Deployment
        name: cert-manager-webhook
        uid: b99471c0-96d6-11e9-bafd-0242a54f69f8
spec:
    group: agroup
    names:
        kind: akind
    version: aversion
`
	obj := map[string]interface{}{}
	err := yaml.Unmarshal([]byte(manifest), obj)
	require.NoError(t, err)
	o := NewK8sObject(obj)
	assert.Equal(t, "certmanager.k8s.io/v1alpha1", o.APIVersion, "apiVersion")
	assert.Equal(t, "Issuer", o.Kind, "kind")
	assert.Equal(t, "ca-issuer", o.Name, "name")
	assert.Equal(t, "cert-manager", o.Namespace, "namespace")
	assert.Equal(t, "agroup/aversion/akind", o.CrdGvk(), "crd group/version/kind")
	assert.Equal(t, "apps/v1", o.OwnerReferences()[0].APIVersion, "ownerReferences[0].apiVersion")
	assert.Equal(t, "Deployment", o.OwnerReferences()[0].Kind, "ownerReferences[0].kind")
	assert.Equal(t, "cert-manager-webhook", o.OwnerReferences()[0].Name, "ownerReferences[0].name")

	var buf bytes.Buffer
	err = o.WriteYaml(&buf)
	require.NoError(t, err)
	assert.Equal(t, "---\n"+manifest, buf.String(), "yamlIn->obj->yamlOut == yamlIn")
}

func TestK8sObjectFromReader(t *testing.T) {
	f, err := os.Open("test/k8sobjectlist.yaml")
	require.NoError(t, err)
	defer f.Close()
	ol, err := K8sObjectsFromReader(f)
	require.NoError(t, err)
	names := []string{}
	for _, o := range ol {
		names = append(names, o.Name)
	}
	expectedNames := []string{"certificates.certmanager.k8s.io", "somedeployment", "myapiservice", "mydeployment", "onemorecert", "cert-manager-webhook"}
	assert.Equal(t, expectedNames, names, "flattened object names")
}
