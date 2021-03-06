package resource

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	l, err := FromReader(bytes.NewReader([]byte(manifest)))
	require.NoError(t, err)
	require.Equal(t, 1, len(l), "list size")
	o := l[0]
	assert.Equal(t, "certmanager.k8s.io/v1alpha1", o.APIVersion(), "apiVersion")
	assert.Equal(t, "Issuer", o.Kind(), "kind")
	assert.Equal(t, "ca-issuer", o.Name(), "name")
	assert.Equal(t, "cert-manager", o.Namespace(), "namespace")
	assert.Equal(t, "issuer.certmanager.k8s.io:cert-manager:ca-issuer", o.ID(), "ID()")
	assert.Equal(t, "issuer.certmanager.k8s.io", o.QualifiedKind(), "QualifiedKind()")
	if assert.NotNil(t, o.Labels(), "labels") {
		assert.Equal(t, "mypkg", o.Labels()["app.kubernetes.io/part-of"], "label")
	}
	require.Equal(t, 2, len(o.Conditions()), "condition amount")
	assert.Equal(t, "available", o.Conditions()[0].Type, "condition[0].type")
	assert.True(t, o.Conditions()[0].Status, "condition[0].status")
	assert.Equal(t, "somecondition", o.Conditions()[1].Type, "condition[1].type")
	assert.False(t, o.Conditions()[1].Status, "condition[1].status")
	assert.Equal(t, "DoesntWork", o.Conditions()[1].Reason, "condition[0].reason")
	assert.Equal(t, "error msg", o.Conditions()[1].Message, "condition[0].message")

	var buf bytes.Buffer
	err = o.WriteYaml(&buf)
	require.NoError(t, err)
	assert.Equal(t, "---\n"+manifest, buf.String(), "yamlIn->obj->yamlOut == yamlIn")
}

func TestFromJsonStream(t *testing.T) {
	f, err := os.Open("../client/mock/watch.json")
	require.NoError(t, err)
	defer f.Close()
	expectedNames := map[string]bool{"mydeployment": true, "otherdeployment": true, "somedeployment-pod-x": true}
	actualNames := map[string]bool{}
	count := 0
	for evt := range FromJsonStream(f) {
		require.NoError(t, evt.Error)
		require.True(t, len(evt.Resource.Conditions()) > 0, "conditions > 0. item: %#v", evt.Resource)
		actualNames[evt.Resource.Name()] = true
		count++
	}
	require.Equal(t, expectedNames, actualNames, "names")
}
