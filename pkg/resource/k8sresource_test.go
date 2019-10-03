package resource

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestK8sResource(t *testing.T) {
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
  uid: someuid
spec:
  group: agroup
  names:
    kind: akind
  version: aversion
status:
  conditions:
  - status: "True"
    type: Available
  - message: error msg
    reason: DoesntWork
    status: "False"
    type: SomeCondition
`
	obj := map[string]interface{}{}
	err := yaml.Unmarshal([]byte(manifest), obj)
	require.NoError(t, err)
	o := FromMap(obj)
	assert.Equal(t, "certmanager.k8s.io/v1alpha1", o.APIVersion, "apiVersion")
	assert.Equal(t, "Issuer", o.Kind, "kind")
	assert.Equal(t, "ca-issuer", o.Name, "name")
	assert.Equal(t, "someuid", o.Uid, "uid")
	assert.Equal(t, "cert-manager", o.Namespace, "namespace")
	assert.Equal(t, "apps/v1", o.OwnerReferences()[0].APIVersion, "ownerReferences[0].apiVersion")
	assert.Equal(t, "Deployment", o.OwnerReferences()[0].Kind, "ownerReferences[0].kind")
	assert.Equal(t, "cert-manager-webhook", o.OwnerReferences()[0].Name, "ownerReferences[0].name")
	assert.Equal(t, "cert-manager/certmanager.k8s.io/v1alpha1/Issuer/ca-issuer", o.ID(), "ID()")
	assert.Equal(t, "certmanager.k8s.io/v1alpha1/Issuer", o.Gvk(), "Gvk()")
	assert.Equal(t, "agroup/aversion/akind", o.CrdGvk(), "crd group/version/kind")
	if assert.NotNil(t, o.Labels(), "labels") {
		assert.Equal(t, "mypkg", o.Labels()["app.kubernetes.io/part-of"], "label")
	}
	require.Equal(t, 2, len(o.Conditions), "condition amount")
	assert.Equal(t, "available", o.Conditions[0].Type, "condition[0].type")
	assert.True(t, o.Conditions[0].Status, "condition[0].status")
	assert.Equal(t, "somecondition", o.Conditions[1].Type, "condition[1].type")
	assert.False(t, o.Conditions[1].Status, "condition[1].status")
	assert.Equal(t, "DoesntWork", o.Conditions[1].Reason, "condition[0].reason")
	assert.Equal(t, "error msg", o.Conditions[1].Message, "condition[0].message")

	var buf bytes.Buffer
	err = o.WriteYaml(&buf)
	require.NoError(t, err)
	assert.Equal(t, "---\n"+manifest, buf.String(), "yamlIn->obj->yamlOut == yamlIn")
}
