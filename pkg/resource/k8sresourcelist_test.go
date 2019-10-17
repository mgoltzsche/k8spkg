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
	ol, err := FromYaml(f)
	require.NoError(t, err)
	names := []string{}
	for _, o := range ol {
		names = append(names, o.Name())
	}
	expectedNames := []string{"certificates.certmanager.k8s.io", "somedeployment", "somedeployment-pod-x", "myapiservice", "mydeployment", "onemorecert", "cert-manager-webhook"}
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
