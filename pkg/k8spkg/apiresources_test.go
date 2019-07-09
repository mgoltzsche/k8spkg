package k8spkg

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	kubectlResTypeCall = "api-resources --verbs delete"
)

var (
	resTypes              = []string{"cm", "ns", "po", "podtemplates", "deploy.apps", "deploy.extensions", "crd.apiextensions.k8s.io"}
	resTypesStr           = strings.Join(resTypes, ",")
	namespacedResTypesStr = "cm,po,podtemplates,deploy.apps,deploy.extensions"
	resTypeTable          = `NAME                              SHORTNAMES   APIGROUP                       NAMESPACED   KIND
configmaps                        cm                                          true         ConfigMap
namespaces                        ns                                          false        Namespace
pods                              po                                          true         Pod
podtemplates                                                                  true         PodTemplate
deployments                       deploy       apps                           true         Deployment
deployments                       deploy       extensions                     true         Deployment
customresourcedefinitions         crd,crds     apiextensions.k8s.io           false        CustomResourceDefinition
`
)

func TestLoadApiResourceTypes(t *testing.T) {
	ctx := context.Background()
	expectedCalls := []string{kubectlResTypeCall}
	// with kubectl success
	assertKubectlCalls(t, expectedCalls, len(expectedCalls), func(_ string) {
		types, err := LoadAPIResourceTypes(ctx)
		require.NoError(t, err)
		require.Equal(t, typeNames(types), resTypes)
	})
	// with kubectl failure
	assertKubectlCalls(t, expectedCalls, 0, func(_ string) {
		_, err := LoadAPIResourceTypes(ctx)
		require.Error(t, err)
	})
}

func TestParseApiResourceTable(t *testing.T) {
	types, err := parseApiResourceTable(bytes.NewReader([]byte(resTypeTable)))
	require.NoError(t, err)

	typeMap := map[string]*APIResourceType{}
	typeNames := make([]string, len(types))
	for i, rtype := range types {
		require.NotEmpty(t, rtype.Name, "name")
		require.NotEmpty(t, rtype.Kind, "kind")
		typeMap[rtype.FullName()] = rtype
		typeNames[i] = rtype.FullName()
	}
	require.Equal(t, resTypes, typeNames, "full names")
	for _, name := range resTypes {
		require.NotNil(t, typeMap[name], name)
	}
	for _, name := range []string{"cm", "deploy.apps", "deploy.extensions", "po", "podtemplates"} {
		require.True(t, typeMap[name].Namespaced, "%s: namespaced", name)
	}
	for _, name := range []string{"ns", "crd.apiextensions.k8s.io"} {
		require.False(t, typeMap[name].Namespaced, "%s: namespaced", name)
	}
	require.Equal(t, []string{"crd", "crds"}, typeMap["crd.apiextensions.k8s.io"].ShortNames, "shortNames")
}

func typeNames(l []*APIResourceType) (names []string) {
	for _, rt := range l {
		names = append(names, rt.FullName())
	}
	return
}
