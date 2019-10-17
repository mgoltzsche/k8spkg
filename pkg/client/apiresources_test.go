package client

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

var (
	resTypeTable = `NAME                              SHORTNAMES   APIGROUP                       NAMESPACED   KIND
configmaps                        cm                                          true         ConfigMap
namespaces                        ns                                          false        Namespace
pods                              po,shortname                                true         Pod
podtemplates                                                                  true         PodTemplate
deployments                       deploy       apps                           true         Deployment
`
	expectedResTypes = []*APIResourceType{
		resType("configmaps", []string{"cm"}, "", "ConfigMap", true),
		resType("namespaces", []string{"ns"}, "", "Namespace", false),
		resType("pods", []string{"po", "shortname"}, "", "Pod", true),
		resType("podtemplates", nil, "", "PodTemplate", true),
		resType("deployments", []string{"deploy"}, "apps", "Deployment", true),
	}
)

func TestResourceTypes(t *testing.T) {
	expectedCalls := []string{"api-resources --verbs delete"}
	assertKubectlCalls(t, expectedCalls, []byte(resTypeTable), func(c K8sClient) (err error) {
		r, err := c.ResourceTypes(context.Background())
		if err == nil {
			require.Equal(t, expectedResTypes, r)
		}
		return
	})
}

func TestParseResourceTypeTable(t *testing.T) {
	types, err := parseResourceTypeTable(bytes.NewReader([]byte(resTypeTable)))
	require.NoError(t, err)
	require.Equal(t, expectedResTypes, types)
}

func resType(name string, shortNames []string, apiGroup, kind string, namespaced bool) *APIResourceType {
	return &APIResourceType{
		Name:       name,
		ShortNames: shortNames,
		APIGroup:   apiGroup,
		Kind:       kind,
		Namespaced: namespaced,
	}
}
