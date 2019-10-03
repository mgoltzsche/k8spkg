package resource

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResourceFromReader(t *testing.T) {
	f, err := os.Open("test/k8sobjectlist.yaml")
	require.NoError(t, err)
	defer f.Close()
	ol, err := FromReader(f)
	require.NoError(t, err)
	names := []string{}
	for _, o := range ol {
		names = append(names, o.Name)
	}
	expectedNames := []string{"certificates.certmanager.k8s.io", "somedeployment", "myapiservice", "mydeployment", "onemorecert", "cert-manager-webhook"}
	assert.Equal(t, expectedNames, names, "flattened object names")
}

func TestResourceListWriteYaml(t *testing.T) {
	manifest := ""
	for i := 0; i < 2; i++ {
		manifest += `---
apiVersion: some.api/aversion
kind: SomeKind
metadata:
  name: object` + fmt.Sprintf("%d", i) + `
  namespace: myns
  sth: else
`
	}
	obj := make([]*K8sResource, 2)
	for i := 0; i < 2; i++ {
		obj[i] = FromMap(map[string]interface{}{
			"apiVersion": "some.api/aversion",
			"kind":       "SomeKind",
			"metadata": map[string]interface{}{
				"name":      fmt.Sprintf("object%d", i),
				"namespace": "myns",
				"sth":       "else",
			},
		})
	}
	var buf bytes.Buffer
	err := K8sResourceList(obj).WriteYaml(&buf)
	require.NoError(t, err)
	assert.Equal(t, manifest, buf.String())
}

func TestGroupByNamespace(t *testing.T) {
	testee := K8sResourceList([]*K8sResource{
		{APIVersion: "v1", Kind: "Deployment", Namespace: "ns-a", Name: "name-a"},
		{APIVersion: "v1", Kind: "Deployment", Name: "name-d"},
		{APIVersion: "v1", Kind: "Deployment", Namespace: "ns-a", Name: "name-b"},
		{APIVersion: "v1", Kind: "Deployment", Namespace: "ns-b", Name: "name-c"},
	})
	groups := testee.GroupByNamespace()
	expected := []*K8sResourceGroup{
		{"ns-a", []*K8sResource{
			{APIVersion: "v1", Kind: "Deployment", Namespace: "ns-a", Name: "name-a"},
			{APIVersion: "v1", Kind: "Deployment", Namespace: "ns-a", Name: "name-b"},
		}},
		{"", []*K8sResource{
			{APIVersion: "v1", Kind: "Deployment", Name: "name-d"},
		}},
		{"ns-b", []*K8sResource{
			{APIVersion: "v1", Kind: "Deployment", Namespace: "ns-b", Name: "name-c"},
		}},
	}
	require.Equal(t, expected, groups)
	require.Equal(t, 0, len(K8sResourceList(nil).GroupByNamespace()), "on nil list")
}

func TestGroupByKind(t *testing.T) {
	testee := K8sResourceList([]*K8sResource{
		{APIVersion: "v1", Kind: "Deployment", Name: "name-a"},
		{APIVersion: "v1", Kind: "Deployment", Name: "name-b"},
		{APIVersion: "v1", Kind: "Secret", Name: "name-c"},
	})
	groups := testee.GroupByKind()
	expected := []*K8sResourceGroup{
		{"Deployment", []*K8sResource{
			{APIVersion: "v1", Kind: "Deployment", Name: "name-a"},
			{APIVersion: "v1", Kind: "Deployment", Name: "name-b"},
		}},
		{"Secret", []*K8sResource{
			{APIVersion: "v1", Kind: "Secret", Name: "name-c"},
		}},
	}
	require.Equal(t, expected, groups)
	require.Equal(t, 0, len(K8sResourceList(nil).GroupByKind()), "on nil list")
}

func TestFilter(t *testing.T) {
	testee := K8sResourceList([]*K8sResource{
		{APIVersion: "v1", Kind: "Deployment", Name: "name-a"},
		{APIVersion: "v1", Kind: "Deployment", Name: "name-b"},
		{APIVersion: "v1", Kind: "Secret", Name: "name-a"},
		{APIVersion: "v1", Kind: "Secret", Name: "name-c"},
	})
	filter := func(o *K8sResource) bool { return o.Name != "name-a" }
	filtered := testee.Filter(filter)
	expected := K8sResourceList([]*K8sResource{
		{APIVersion: "v1", Kind: "Deployment", Name: "name-b"},
		{APIVersion: "v1", Kind: "Secret", Name: "name-c"},
	})
	require.Equal(t, expected, filtered)
	require.Equal(t, 0, len(K8sResourceList(nil).Filter(filter)), "on nil list")
}
