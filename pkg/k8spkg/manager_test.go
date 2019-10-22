package k8spkg

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/mgoltzsche/k8spkg/pkg/client/mock"
	"github.com/mgoltzsche/k8spkg/pkg/resource"
	"github.com/stretchr/testify/require"
)

func assertPkgManagerCall(t *testing.T, call func(*PackageManager, *mock.ClientMock) error) {
	client := mock.NewClientMock()
	testee := NewPackageManager(client, "")
	err := call(testee, client)
	require.NoError(t, err)
	client.MockErr = fmt.Errorf("error mock")
	err = call(testee, client)
	require.Error(t, err)
	require.Contains(t, err.Error(), client.MockErr.Error(), "error message should contain cause")
}

func TestPackageManagerApply(t *testing.T) {
	f, err := os.Open("../client/mock/watch.json")
	require.NoError(t, err)
	defer f.Close()
	var obj resource.K8sResourceList
	for evt := range resource.FromJsonStream(f) {
		require.NoError(t, evt.Error)
		obj = append(obj, evt.Resource)
	}
	//obj := mock.MockResourceList("../resource/test/k8sobjectlist.yaml")
	pkg := &K8sPackage{&PackageInfo{Name: "somepkg"}, obj}
	labels := fmt.Sprintf("[%s=%s]", PKG_NAME_LABEL, pkg.Name)
	for _, ns := range []string{"", "myns"} {
		expectedCalls := []string{
			fmt.Sprintf("apply %s/ false []", ns),
			fmt.Sprintf("apply %s/ %v %s", ns, false, labels), // TODO: test prune
		}
		for _, byNs := range obj.Refs().GroupByNamespace() {
			for _, byKind := range byNs.Resources.GroupByKind() {
				gns := byNs.Key
				if gns == "" {
					gns = ns
				}
				expectedCalls = append(expectedCalls,
					fmt.Sprintf("watch %s/%s %s", gns, byKind.Key, labels))
			}
		}
		expectedCalls = append(expectedCalls,
			"watch otherns/Pod [app=otherdeployment otherlabel=otherval]",
		)
		obj[len(obj)-1].Conditions()[0].Status = false
		assertPkgManagerCall(t, func(testee *PackageManager, c *mock.ClientMock) (err error) {
			testee = NewPackageManager(c, ns)
			evts := make([]resource.ResourceEvent, len(obj))
			for i, res := range obj {
				evts[i] = resource.ResourceEvent{res, c.MockErr}
			}
			c.MockWatchEvents = evts
			err = testee.Apply(context.Background(), pkg, false)
			require.Error(t, err, "unavailable (last) deployment should cause error")
			if c.MockErr == nil {
				require.Equal(t, expectedCalls, c.Calls, "client calls")
			}
			evts[len(evts)-1].Resource.Conditions()[0].Status = true
			c.Calls = c.Calls[:0]
			c.Applied = nil
			if err = testee.Apply(context.Background(), pkg, false); err == nil {
				require.Equal(t, obj, c.Applied, "applied")
				require.Equal(t, expectedCalls[:len(expectedCalls)-1], c.Calls[:len(expectedCalls)-1], "client calls")
			}
			return
		})
	}
}

func TestPackageManagerList(t *testing.T) {
	testApp2 := *testApp
	testApp2.Name = testApp.Name + "2"
	appRes := testAppResource(t, testApp, &testApp2)
	for _, ns := range []string{"", "myns"} {
		expectedCalls := []string{
			fmt.Sprintf("get %s/ %s.%s []", ns, strings.ToLower(CrdKind), CrdAPIGroup),
		}
		assertPkgManagerCall(t, func(testee *PackageManager, c *mock.ClientMock) (err error) {
			c.MockResources = appRes
			apps, err := testee.List(context.Background(), ns)
			if err == nil {
				require.Equal(t, []*App{testApp, &testApp2}, apps, "retrieved")
				require.Equal(t, expectedCalls, c.Calls, "client calls")
			}
			return
		})
	}
}

func TestPackageManagerDelete(t *testing.T) {
	for _, ns := range []string{"", "myns"} {
		expectedCalls := []string{
			fmt.Sprintf("getresource %s/ %s.%s %s", ns, strings.ToLower(CrdKind), CrdAPIGroup, testApp.Name),
			fmt.Sprintf("delete %s/ [deployment.apps/mydeployment apiservice.apiservice/myapi]", ns),
			fmt.Sprintf("awaitdeletion %s/ [deployment.apps/mydeployment apiservice.apiservice/myapi]", ns),
			fmt.Sprintf("delete %s/ [%s.%s/%s]", testApp.Namespace, strings.ToLower(CrdKind), CrdAPIGroup, testApp.Name),
		}
		assertPkgManagerCall(t, func(testee *PackageManager, c *mock.ClientMock) (err error) {
			testee = NewPackageManager(c, ns)
			c.MockResource = testAppResource(t, testApp)[0]
			err = testee.Delete(context.Background(), testApp.Name)
			if err == nil {
				require.Equal(t, expectedCalls, c.Calls, "client calls")
			}
			return
		})
	}
}
