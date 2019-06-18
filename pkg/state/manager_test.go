package state

import (
	"bytes"
	"io/ioutil"
	"testing"

	"github.com/mgoltzsche/k8spkg/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPackageManagerState(t *testing.T) {
	expectedCalls := []string{
		resTypeCallCluster,
		resTypeCallNamespaced,
		"get " + resTypesStr + " --all-namespaces -l app.kubernetes.io/part-of=somepkg -o yaml",
	}
	// with kubectl success
	assertKubectlCalls(t, expectedCalls, len(expectedCalls), func(_ string) {
		testee := NewPackageManager()
		objects, err := testee.State("somepkg")
		require.NoError(t, err)
		assert.Equal(t, 5, len(objects), "amount of loaded objects")
	})
	// with kubectl error
	assertKubectlCalls(t, expectedCalls[:1], 0, func(_ string) {
		testee := NewPackageManager()
		_, err := testee.State("somepkg")
		assert.Error(t, err)
	})
	assertKubectlCalls(t, expectedCalls, 2, func(_ string) {
		testee := NewPackageManager()
		_, err := testee.State("somepkg")
		assert.Error(t, err)
	})
}

func TestPackageManagerDelete(t *testing.T) {
	expectedCalls := []string{
		resTypeCallCluster,
		resTypeCallNamespaced,
		"get " + resTypesStr + " --all-namespaces -l app.kubernetes.io/part-of=somepkg -o yaml",
		"delete --wait=true --timeout=2m --cascade=true --ignore-not-found=true -n cert-manager certificate/onemorecert",
		"wait --for delete --timeout=2m -n cert-manager certificate/onemorecert",
		"delete --wait=true --timeout=2m --cascade=true --ignore-not-found=true -n mynamespace deployment/somedeployment deployment/mydeployment",
		"wait --for delete --timeout=2m -n mynamespace deployment/somedeployment deployment/mydeployment",
		"delete --wait=true --timeout=2m --cascade=true --ignore-not-found=true apiservice/myapiservice",
		"wait --for delete --timeout=2m apiservice/myapiservice",
		"delete --wait=true --timeout=2m --cascade=true --ignore-not-found=true customresourcedefinition/certificates.certmanager.k8s.io",
		"wait --for delete --timeout=2m customresourcedefinition/certificates.certmanager.k8s.io",
	}
	// successful deletion
	assertKubectlCalls(t, expectedCalls, len(expectedCalls), func(_ string) {
		testee := NewPackageManager()
		assert.NoError(t, testee.Delete("somepkg"), "should delete successfully")
	})
	// kubectl error on state retrieval should fail
	assertKubectlCalls(t, expectedCalls[:1], 0, func(_ string) {
		testee := NewPackageManager()
		require.Error(t, testee.Delete("somepkg"), "should fail when resource type retrieval fails")
	})
	assertKubectlCalls(t, expectedCalls[:3], 2, func(_ string) {
		testee := NewPackageManager()
		require.Error(t, testee.Delete("somepkg"), "should fail when cluster state retrieval fails")
	})
	// kubectl error during deletion should still delete other resources
	/*expectedCalls = append(expectedCalls, resTypeCallCluster, resTypeCallNamespaced, expectedCalls[2])
	assertKubectlCalls(t, expectedCalls, 3, func(_ string) {
		testee := NewPackageManager()
		require.Error(t, testee.Delete("somepkg"))
	})*/
}

func TestPackageManagerApply(t *testing.T) {
	b, err := ioutil.ReadFile("../model/test/k8sobjectlist.yaml")
	require.NoError(t, err)
	obj, err := model.K8sObjectsFromReader(bytes.NewReader(b))
	require.NoError(t, err)

	// Assert Apply()
	expectedCalls := []string{
		"apply --wait=true --prune -f - -l app.kubernetes.io/part-of=testpkg",
		"rollout status -w --timeout=2m -n mynamespace deployment/somedeployment",
		"rollout status -w --timeout=2m -n mynamespace deployment/mydeployment",
		"wait --for condition=available --timeout=2m -n mynamespace deployment/somedeployment deployment/mydeployment",
		"wait --for condition=available --timeout=2m apiservice/myapiservice",
	}
	assertKubectlCalls(t, expectedCalls, len(expectedCalls), func(stdinFile string) {
		testee := &PackageManager{}
		err = testee.Apply(&model.K8sPackage{"testpkg", obj})
		require.NoError(t, err, "Apply()")

		// Assert applied content is complete
		expected, err := model.K8sObjectsFromReader(bytes.NewReader(b))
		require.NoError(t, err)
		var expectedYaml bytes.Buffer
		for _, o := range expected {
			m := o.Metadata()
			// see https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/
			m.Labels["app.kubernetes.io/managed-by"] = "k8spkg"
			m.Labels["app.kubernetes.io/part-of"] = "testpkg"
			o.SetMetadata(m)
			o.WriteYaml(&expectedYaml)
		}
		appliedYaml, err := ioutil.ReadFile(stdinFile)
		require.NoError(t, err)
		assert.Equal(t, expectedYaml.String(), string(appliedYaml), "withLabels(in) == out")
	})

	// Assert kubectl error is passed through
	expectedCalls = []string{
		"apply --wait=true --prune -f - -l app.kubernetes.io/part-of=errorneous-manifests",
	}
	assertKubectlCalls(t, expectedCalls, 0, func(_ string) {
		testee := NewPackageManager()
		err := testee.Apply(model.NewK8sPackage("errorneous-manifests", obj))
		require.Error(t, err, "Apply() should pass through kubectl error")
	})
}
