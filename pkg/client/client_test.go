package client

import (
	"bytes"
	"context"
	"fmt"
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
	mockOut, err := ioutil.ReadFile("mock/get-list.json")
	require.NoError(t, err)
	obj, err := resource.FromReader(bytes.NewReader(mockOut))
	require.NoError(t, err)
	labelCases := [][]string{nil, {"my/label1=val1", "my/label2=val2"}}
	for _, labels := range labelCases {
		for _, ns := range []string{"", "myns"} {
			expectedCall := "apply --wait -f - --record --timeout=" + defaultTimeout.String()
			if len(labels) > 0 {
				expectedCall += " -l " + strings.Join(labels, ",")
			}
			if ns != "" {
				expectedCall += " -n " + ns
			}
			expectedCall += " -o json"
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
	refs := append(obj.Refs(), resource.ResourceRef("v1", "Secret", "cert-manager", "mysecret"))
	for _, ns := range []string{"", "myns"} {
		expectedCalls := []string{}
		for _, grp := range refs.GroupByNamespace() {
			expectedCall := "delete --wait --cascade --ignore-not-found --timeout=" + defaultTimeout.String()
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
	expectedCall := "get --ignore-not-found deployment " + deploymentName
	for _, ns := range []string{"", "myns"} {
		expectedCallOpts := expectedCall
		if ns != "" {
			expectedCallOpts += " -n " + ns
		}
		expectedCallOpts += " -o json"
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
	expectedCall += " -o json"
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
			expectedCall := "get pod,deployment"
			if len(labels) > 0 {
				expectedCall += " -l " + strings.Join(labels, ",")
			}
			if ns != "" {
				expectedCall += " -n " + ns
			}
			expectedCall += " -o json"
			expectedCalls := []string{expectedCall}
			assertKubectlCalls(t, expectedCalls, mockOut, func(c K8sClient) (err error) {
				kinds := []string{"Pod", "Deployment"}
				var res resource.K8sResourceList
				for evt := range c.Get(context.Background(), kinds, ns, labels) {
					if evt.Error != nil {
						if err == nil {
							err = evt.Error
						}
						continue
					}
					res = append(res, evt.Resource)
				}
				if err == nil {
					require.Equal(t, testDeploymentNames, res.Refs().Names())
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
			for _, watchOnly := range []bool{false, true} {
				expectedCall := "get -w deployment"
				if len(labels) > 0 {
					expectedCall += " -l " + strings.Join(labels, ",")
				}
				if watchOnly {
					expectedCall += " --watch-only"
				}
				if ns != "" {
					expectedCall += " -n " + ns
				}
				expectedCall += " -o json"
				expectedCalls := []string{expectedCall}
				assertKubectlCalls(t, expectedCalls, mockOut, func(c K8sClient) (err error) {
					resNames := map[string]bool{}
					returnedIds := []string{}
					for evt := range c.Watch(context.Background(), "Deployment", ns, labels, watchOnly) {
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
}

func TestAwaitDeletion(t *testing.T) {
	var res resource.K8sResourceRefList
	names := [2]string{}
	for i := 0; i < 3; i++ {
		name := fmt.Sprintf("deploy-%d", i)
		ns := fmt.Sprintf("ns-%d", i%2)
		res = append(res, resource.ResourceRef("apps/v1", "Deployment", ns, name))
		names[i%2] += " deployment.apps/" + name
	}
	expectedCall := "wait --for delete --timeout=" + defaultTimeout.String()
	expectedCalls := []string{
		expectedCall + names[0] + " -n ns-0",
		expectedCall + names[1] + " -n ns-1",
	}
	assertKubectlCalls(t, expectedCalls, nil, func(c K8sClient) (err error) {
		return c.AwaitDeletion(context.Background(), "", res)
	})
}
