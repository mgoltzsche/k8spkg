package k8spkg

import (
	"bytes"
	"context"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/mgoltzsche/k8spkg/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	kubectlApplyCall = "apply --wait=true --timeout=2m -f - -l app.kubernetes.io/part-of=somepkg --prune"
)

func TestPackageManagerApply(t *testing.T) {
	b, err := ioutil.ReadFile("../model/test/k8sobjectlist.yaml")
	require.NoError(t, err)
	obj, err := model.FromReader(bytes.NewReader(b))
	require.NoError(t, err)
	pkg := &K8sPackage{&PackageInfo{Name: "somepkg"}, obj}

	// Assert Apply()
	expectedCalls := []string{
		kubectlApplyCall,
		"rollout status -w --timeout=2m -n mynamespace deployment/somedeployment",
		"rollout status -w --timeout=2m -n mynamespace deployment/mydeployment",
		"rollout status -w --timeout=2m -n cert-manager deployment/cert-manager-webhook",
		"wait --for condition=available --timeout=2m -n mynamespace deployment/somedeployment deployment/mydeployment",
		"wait --for condition=available --timeout=2m apiservice/myapiservice",
		"wait --for condition=available --timeout=2m -n cert-manager deployment/cert-manager-webhook",
	}
	assertKubectlCalls(t, expectedCalls, len(expectedCalls), func(stdinFile string) {
		testee := &PackageManager{}
		err = testee.Apply(context.Background(), pkg, true)
		require.NoError(t, err, "Apply()")

		// Assert applied content is complete
		expected, err := model.FromReader(bytes.NewReader(b))
		require.NoError(t, err)
		var expectedYaml bytes.Buffer
		for _, o := range expected {
			o.WriteYaml(&expectedYaml)
		}
		appliedYaml, err := ioutil.ReadFile(stdinFile)
		require.NoError(t, err)
		assert.Equal(t, expectedYaml.String(), string(appliedYaml), "withLabels(in) == out")
	})

	// Assert prune option and kubectl error are passed through
	expectedCalls = []string{"apply --wait=true --timeout=2m -f -"}
	assertKubectlCalls(t, expectedCalls, 0, func(_ string) {
		testee := NewPackageManager()
		err := testee.Apply(context.Background(), pkg, false)
		require.Error(t, err, "Apply() should pass through kubectl error")
	})
}

var testNamespace = "myns"
var kubectlGetCall = "get " + resTypesStr + " -l app.kubernetes.io/part-of=somepkg -o yaml -n " + testNamespace
var kubectlGetCallNsCertManager = "get " + namespacedResTypesStr + " -l app.kubernetes.io/part-of=somepkg -o yaml -n cert-manager"
var kubectlGetCallNsMynamespace = "get " + namespacedResTypesStr + " -l app.kubernetes.io/part-of=somepkg -o yaml -n mynamespace"
var kubectlGetCallNsEmpty = "get " + resTypesStr + " -l app.kubernetes.io/part-of=somepkg -o yaml"

func TestPackageManagerState(t *testing.T) {
	expectedCalls := []string{
		kubectlResTypeCall,
		kubectlGetCall,
		kubectlGetCallNsCertManager,
		kubectlGetCallNsMynamespace,
	}
	// with kubectl success
	assertns := func(ns string) func(string) {
		return func(_ string) {
			testee := NewPackageManager()
			pkg, err := testee.State(context.Background(), ns, "somepkg")
			require.NoError(t, err)
			require.Equal(t, "somepkg", pkg.Name, "pkg name")
			s := ""
			for _, o := range pkg.Objects {
				s += "\n  " + o.ID()
			}
			require.Equal(t, 9, len(pkg.Objects), "amount of loaded objects\nobjects: %s", s)
		}
	}
	assertKubectlCalls(t, expectedCalls, len(expectedCalls), assertns(testNamespace))
	expectedCalls[1] = kubectlGetCallNsEmpty
	assertKubectlCalls(t, expectedCalls[:3], 3, assertns(""))
	// with kubectl error
	assertKubectlCalls(t, expectedCalls[:1], 0, func(_ string) {
		testee := NewPackageManager()
		_, err := testee.State(context.Background(), "", "somepkg")
		assert.Error(t, err)
	})
	/*assertKubectlCalls(t, expectedCalls[:2], 2, func(_ string) {
		testee := NewPackageManager()
		_, err := testee.State(context.Background(), "", "somepkg")
		assert.Error(t, err)
	})*/
}

func TestPackageManagerDelete(t *testing.T) {
	expectedCalls := []string{
		kubectlResTypeCall,
		kubectlGetCall,
		kubectlGetCallNsCertManager,
		kubectlGetCallNsMynamespace,
		"delete --wait=true --timeout=2m --cascade=true --ignore-not-found=true -n cert-manager certificate/onemorecert certificate/mycert",
		"wait --for delete --timeout=2m -n cert-manager certificate/onemorecert certificate/mycert",
		"delete --wait=true --timeout=2m --cascade=true --ignore-not-found=true -n mynamespace deployment/somedeployment deployment/mydeployment",
		"delete --wait=true --timeout=2m --cascade=true --ignore-not-found=true -n cert-manager deployment/cert-manager-webhook",
		"wait --for delete --timeout=2m -n mynamespace deployment/somedeployment deployment/mydeployment",
		"wait --for delete --timeout=2m -n cert-manager deployment/cert-manager-webhook replicaset/cert-manager-webhook-7444b58c45 pod/cert-manager-webhook-7444b58c45-9cfgh",
		"delete --wait=true --timeout=2m --cascade=true --ignore-not-found=true apiservice/myapiservice",
		"wait --for delete --timeout=2m apiservice/myapiservice",
		"delete --wait=true --timeout=2m --cascade=true --ignore-not-found=true customresourcedefinition/certificates.certmanager.k8s.io",
		"wait --for delete --timeout=2m customresourcedefinition/certificates.certmanager.k8s.io",
	}
	// successful deletion
	assertKubectlCalls(t, expectedCalls, len(expectedCalls), func(_ string) {
		testee := NewPackageManager()
		require.NoError(t, testee.Delete(context.Background(), "myns", "somepkg"), "should delete successfully")
	})
	// kubectl error on state retrieval should fail
	assertKubectlCalls(t, expectedCalls[:1], 0, func(_ string) {
		testee := NewPackageManager()
		require.Error(t, testee.Delete(context.Background(), "myns", "somepkg"), "should fail when resource type retrieval fails")
	})
	assertKubectlCalls(t, expectedCalls[:2], 1, func(_ string) {
		testee := NewPackageManager()
		require.Error(t, testee.Delete(context.Background(), "myns", "somepkg"), "should fail when cluster state retrieval fails")
	})
	assertKubectlCalls(t, expectedCalls[:3], 2, func(_ string) {
		testee := NewPackageManager()
		require.Error(t, testee.Delete(context.Background(), "myns", "somepkg"), "should fail when cluster state retrieval fails")
	})
	// kubectl error during deletion should still attempt to delete other resources
	expectedCalls = append(expectedCalls, expectedCalls[0])
	assertKubectlCalls(t, expectedCalls, 4, func(_ string) {
		testee := NewPackageManager()
		require.Error(t, testee.Delete(context.Background(), "myns", "somepkg"))
	})
}

var kubectlListCall = "get " + resTypesStr + " -o yaml -n " + testNamespace
var kubectlListCallNsEmpty = "get " + resTypesStr + " -o yaml"
var kubectlListCallAllNamespaces = "get " + resTypesStr + " -o yaml --all-namespaces"

func TestPackageManagerList(t *testing.T) {
	expectedCalls := []string{
		kubectlResTypeCall,
		kubectlListCall,
	}
	// with kubectl success
	assertns := func(allNamespaces bool, ns string) func(string) {
		return func(_ string) {
			testee := NewPackageManager()
			pkgs, err := testee.List(context.Background(), allNamespaces, ns)
			require.NoError(t, err)
			names := make([]string, len(pkgs))
			namespaces := make([]string, len(pkgs))
			for i, p := range pkgs {
				names[i] = p.Name
				namespaces[i] = strings.Join(p.Namespaces, ".")
			}
			require.Equal(t, []string{"pkg-othernamespace", "somepkg"}, names, "package list")
			require.Equal(t, []string{"othernamespace", "cert-manager.mynamespace"}, namespaces, "package namespaces")
		}
	}
	assertKubectlCalls(t, expectedCalls, len(expectedCalls), assertns(false, testNamespace))
	expectedCalls[1] = kubectlListCallNsEmpty
	assertKubectlCalls(t, expectedCalls, len(expectedCalls), assertns(false, ""))
	expectedCalls = []string{
		kubectlResTypeCall,
		kubectlListCallAllNamespaces,
	}
	assertKubectlCalls(t, expectedCalls, len(expectedCalls), assertns(true, ""))
	// with kubectl error
	assertKubectlCalls(t, expectedCalls[:1], 0, func(_ string) {
		testee := NewPackageManager()
		_, err := testee.State(context.Background(), "", "somepkg")
		assert.Error(t, err)
	})
}
