package resource

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGroupByNamespace(t *testing.T) {
	testee := K8sResourceRefList([]K8sResourceRef{
		ResourceRef("", "v1", "Deployment", "ns-a", "name-a"),
		ResourceRef("", "v1", "Deployment", "", "name-d"),
		ResourceRef("", "v1", "Deployment", "ns-a", "name-b"),
		ResourceRef("", "v1", "Deployment", "ns-b", "name-c"),
	})
	groups := testee.GroupByNamespace()
	expected := []*K8sResourceGroup{
		{"ns-a", []K8sResourceRef{
			ResourceRef("", "v1", "Deployment", "ns-a", "name-a"),
			ResourceRef("", "v1", "Deployment", "ns-a", "name-b"),
		}},
		{"", []K8sResourceRef{
			ResourceRef("", "v1", "Deployment", "", "name-d"),
		}},
		{"ns-b", []K8sResourceRef{
			ResourceRef("", "v1", "Deployment", "ns-b", "name-c"),
		}},
	}
	require.Equal(t, expected, groups)
	require.Equal(t, 0, len(K8sResourceRefList(nil).GroupByNamespace()), "on nil list")
}

func TestGroupByKind(t *testing.T) {
	testee := K8sResourceRefList([]K8sResourceRef{
		ResourceRef("", "v1", "Deployment", "", "name-a"),
		ResourceRef("", "v1", "Deployment", "", "name-b"),
		ResourceRef("", "v1", "Secret", "", "name-c"),
	})
	groups := testee.GroupByKind()
	expected := []*K8sResourceGroup{
		{"Deployment", []K8sResourceRef{
			ResourceRef("", "v1", "Deployment", "", "name-a"),
			ResourceRef("", "v1", "Deployment", "", "name-b"),
		}},
		{"Secret", []K8sResourceRef{
			ResourceRef("", "v1", "Secret", "", "name-c"),
		}},
	}
	require.Equal(t, expected, groups)
	require.Equal(t, 0, len(K8sResourceRefList(nil).GroupByKind()), "on nil list")
}

func TestFilter(t *testing.T) {
	testee := K8sResourceRefList([]K8sResourceRef{
		ResourceRef("", "v1", "Deployment", "", "name-a"),
		ResourceRef("", "v1", "Deployment", "", "name-b"),
		ResourceRef("", "v1", "Secret", "", "name-a"),
		ResourceRef("", "v1", "Secret", "", "name-c"),
	})
	filter := func(o K8sResourceRef) bool { return o.Name() != "name-a" }
	filtered := testee.Filter(filter)
	expected := K8sResourceRefList([]K8sResourceRef{
		ResourceRef("", "v1", "Deployment", "", "name-b"),
		ResourceRef("", "v1", "Secret", "", "name-c"),
	})
	require.Equal(t, expected, filtered)
	require.Equal(t, 0, len(K8sResourceRefList(nil).Filter(filter)), "on nil list")
}
