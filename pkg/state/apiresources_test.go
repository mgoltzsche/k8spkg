package state

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	resTypeCallNamespaced = "api-resources --verbs delete --namespaced=true -o name"
	resTypeCallCluster    = "api-resources --verbs delete --namespaced=false -o name"
)

var (
	resTypesNamespaced = []string{"pods", "deployments.apps", "certificates.certmanager.k8s.io"}
	resTypesCluster    = []string{"namespaces", "apiservices.apiregistration.k8s.io", "clusterissuers.certmanager.k8s.io"}
	resTypes           = append(resTypesCluster, resTypesNamespaced...)
	resTypesStr        = strings.Join(resTypes, ",")
)

func TestApiResources(t *testing.T) {
	expectedCalls := []string{
		resTypeCallCluster,
		resTypeCallNamespaced,
	}
	// with kubectl success
	assertKubectlCalls(t, expectedCalls, len(expectedCalls), func(_ string) {
		testee := &ApiResourceTypes{}
		cluster, err := testee.Cluster()
		require.NoError(t, err, "Cluster()")
		namespaced, err := testee.Namespaced()
		require.NoError(t, err, "Namespaced()")
		all, err := testee.All()
		require.NoError(t, err, "All()")

		assert.Equal(t, resTypesCluster, cluster, "cluster types")
		assert.Equal(t, resTypesNamespaced, namespaced, "namespaced types")
		assert.Equal(t, resTypes, all, "all types")
	})
}
