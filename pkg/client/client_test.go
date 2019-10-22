package client

import (
	"bytes"
	"context"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/mgoltzsche/k8spkg/pkg/resource"
	"github.com/stretchr/testify/require"
)

var (
	testDeploymentNames = []string{
		"deployment.extensions/cert-manager",
		"deployment.extensions/cert-manager-cainjector",
		"configmap/cert-manager-controller",
		"configmap/controller-leader-election-helper",
	}
)

func TestApply(t *testing.T) {
	mockOut, err := ioutil.ReadFile("../resource/test/k8sobjectlist.yaml")
	require.NoError(t, err)
	obj, err := resource.FromReader(bytes.NewReader(mockOut))
	require.NoError(t, err)
	labelCases := [][]string{nil, {"my/label1=val1", "my/label2=val2"}}
	for _, labels := range labelCases {
		for _, ns := range []string{"", "myns"} {
			expectedCall := "apply -o yaml --wait --timeout=" + defaultTimeout.String() + " -f - --record"
			if len(labels) > 0 {
				expectedCall += " -l " + strings.Join(labels, ",")
			}
			if ns != "" {
				expectedCall += " -n " + ns
			}
			expectedCalls := []string{expectedCall}
			assertKubectlCalls(t, expectedCalls, mockOut, func(c K8sClient) (err error) {
				r, err := c.Apply(context.Background(), ns, obj, false, labels)
				if err == nil {
					require.Equal(t, obj.Refs().Names(), r.Refs().Names(), "applied - result")
				}
				return
			})
		}
	}
}

func TestDelete(t *testing.T) {
	mockOut, err := ioutil.ReadFile("../resource/test/k8sobjectlist.yaml")
	require.NoError(t, err)
	obj, err := resource.FromReader(bytes.NewReader(mockOut))
	require.NoError(t, err)
	refs := append(obj.Refs(), resource.ResourceRef("", "v1", "Secret", "cert-manager", "mysecret"))
	for _, ns := range []string{"", "myns"} {
		expectedCalls := []string{}
		for _, grp := range refs.GroupByNamespace() {
			expectedCall := "delete --wait --timeout=" + defaultTimeout.String() + " --cascade --ignore-not-found"
			for _, o := range grp.Resources {
				expectedCall += " " + o.QualifiedKind() + "/" + o.Name()
			}
			if grp.Key == "" {
				grp.Key = ns
			}
			if grp.Key != "" {
				expectedCall += " -n " + grp.Key
			}
			expectedCalls = append(expectedCalls, expectedCall)
		}
		assertKubectlCalls(t, expectedCalls, nil, func(c K8sClient) (err error) {
			return c.Delete(context.Background(), ns, refs)
		})
	}
}

func TestGetResource(t *testing.T) {
	mockOut, err := ioutil.ReadFile("mock/get.json")
	require.NoError(t, err)
	deploymentName := "cert-manager"
	expectedCall := "get -o json --ignore-not-found deployment " + deploymentName
	for _, ns := range []string{"", "myns"} {
		expectedCallOpts := expectedCall
		if ns != "" {
			expectedCallOpts += " -n " + ns
		}
		expectedCalls := []string{expectedCallOpts}
		assertKubectlCalls(t, expectedCalls, mockOut, func(c K8sClient) (err error) {
			r, err := c.GetResource(context.Background(), "Deployment", ns, deploymentName)
			if err == nil {
				require.Equal(t, deploymentName, r.Name())
			}
			return
		})
	}
	notFoundErr := false
	assertKubectlCalls(t, []string{expectedCall}, []byte("\n"), func(c K8sClient) (err error) {
		_, e := c.GetResource(context.Background(), "Deployment", "", deploymentName)
		if IsNotFound(e) {
			notFoundErr = true
		} else {
			err = e
		}
		return
	})
	require.True(t, notFoundErr, "IsNotFound(err) expected")
}

func TestGet(t *testing.T) {
	mockOut, err := ioutil.ReadFile("mock/get-list.json")
	require.NoError(t, err)
	labelCases := [][]string{nil, {"my/label1=val1", "my/label2=val2"}}
	for _, ns := range []string{"", "myns"} {
		for _, labels := range labelCases {
			expectedCall := "get -o json pod,deployment"
			if len(labels) > 0 {
				expectedCall += " -l " + strings.Join(labels, ",")
			}
			if ns != "" {
				expectedCall += " -n " + ns
			}
			expectedCalls := []string{expectedCall}
			assertKubectlCalls(t, expectedCalls, mockOut, func(c K8sClient) (err error) {
				kinds := []string{"Pod", "Deployment"}
				r, err := c.Get(context.Background(), kinds, ns, labels)
				if err == nil {
					require.Equal(t, testDeploymentNames, r.Refs().Names())
				}
				return
			})
		}
	}
}

func TestWatch(t *testing.T) {
	mockOut, err := ioutil.ReadFile("mock/watch.json")
	require.NoError(t, err)
	labelCases := [][]string{nil, {"my/label1=val1", "my/label2=val2"}}
	for _, ns := range []string{"", "myns"} {
		for _, labels := range labelCases {
			expectedCall := "get -o json -w deployment"
			if len(labels) > 0 {
				expectedCall += " -l " + strings.Join(labels, ",")
			}
			if ns != "" {
				expectedCall += " -n " + ns
			}
			expectedCalls := []string{expectedCall}
			assertKubectlCalls(t, expectedCalls, mockOut, func(c K8sClient) (err error) {
				resNames := map[string]bool{}
				returnedIds := []string{}
				for evt := range c.Watch(context.Background(), "Deployment", ns, labels) {
					if evt.Error != nil {
						err = evt.Error
					}
					if err != nil {
						continue
					}
					if !resNames[evt.Resource.ID()] {
						returnedIds = append(returnedIds, evt.Resource.ID())
					}
					resNames[evt.Resource.ID()] = true
				}
				if err == nil {
					expectedIds := []string{
						"deployment.apps:default:mydeployment",
						"pod:default:somedeployment-pod-x",
						"deployment.apps:otherns:otherdeployment",
					}
					require.Equal(t, expectedIds, returnedIds, "returned")
				}
				return
			})
		}
	}
}
