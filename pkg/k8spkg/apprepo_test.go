package k8spkg

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/mgoltzsche/k8spkg/pkg/client/mock"
	"github.com/mgoltzsche/k8spkg/pkg/resource"
	"github.com/stretchr/testify/require"
)

var testApp = &App{Name: "myapp", Namespace: "myns", Resources: []resource.K8sResourceRef{
	resource.ResourceRef("apps", "v1", "Deployment", "myns", "mydeployment"),
	resource.ResourceRef("apiservice", "v1", "APIService", "", "myapi"),
}}

func assertAppRepoCall(t *testing.T, call func(*AppRepo, *mock.ClientMock) error) {
	client := mock.NewClientMock()
	testee := NewAppRepo(client)
	err := call(testee, client)
	require.NoError(t, err)
	client.MockErr = fmt.Errorf("error mock")
	client.Calls = nil
	err = call(testee, client)
	require.Error(t, err)
	require.Contains(t, err.Error(), client.MockErr.Error(), "error message should contain cause")
}

func testAppResource(t *testing.T, apps ...*App) resource.K8sResourceList {
	appRes := resource.K8sResourceList(make([]*resource.K8sResource, len(apps)))
	for i, app := range apps {
		ref := resource.ResourceRef(CrdAPIGroup, CrdAPIVersion, CrdKind, app.Namespace, app.Name)
		res := make([]interface{}, len(app.Resources))
		for j, r := range app.Resources {
			apiVersion := r.APIVersion()
			apiGroup := r.APIGroup()
			if apiGroup != "" {
				apiVersion = apiGroup + "/" + apiVersion
			}
			res[j] = map[string]interface{}{
				"apiVersion": apiVersion,
				"kind":       r.Kind(),
				"name":       r.Name(),
				"namespace":  r.Namespace(),
			}
		}
		appRes[i] = resource.Resource(ref, map[string]interface{}{"spec": map[string]interface{}{"resources": res}})
	}
	return appRes
}

func TestAppRepoPut(t *testing.T) {
	res := testAppResource(t, testApp)
	expectedCalls := []string{
		fmt.Sprintf("apply myns/ false []"),
	}
	assertAppRepoCall(t, func(testee *AppRepo, c *mock.ClientMock) (err error) {
		if err = testee.Put(context.Background(), testApp); err == nil {
			require.Equal(t, res, c.Applied, "put")
			require.Equal(t, expectedCalls, c.Calls, "client calls")
		}
		return
	})
}

func TestAppRepoGet(t *testing.T) {
	kind := strings.ToLower(CrdKind) + "." + CrdAPIGroup
	expectedCalls := []string{
		fmt.Sprintf("getresource %s/ %s %s", testApp.Namespace, kind, testApp.Name),
	}
	res := testAppResource(t, testApp)
	assertAppRepoCall(t, func(testee *AppRepo, c *mock.ClientMock) (err error) {
		c.MockResource = res[0]
		retrieved, err := testee.Get(context.Background(), testApp.Namespace, testApp.Name)
		if err == nil {
			require.Equal(t, testApp, retrieved, "get")
			require.Equal(t, expectedCalls, c.Calls, "client calls")
		}
		return
	})
}

func TestAppRepoGetAll(t *testing.T) {
	testApp2 := &App{
		Name:      "anotherapp",
		Namespace: "myns",
		Resources: []resource.K8sResourceRef{
			resource.ResourceRef("apps", "v1", "Deployment", "", "anotherdeployment"),
		},
	}
	expectedCalls := []string{
		fmt.Sprintf("get myns/ %s.%s []", strings.ToLower(CrdKind), CrdAPIGroup),
	}
	expectedApps := []*App{
		testApp,
		testApp2,
	}
	res := testAppResource(t, testApp, testApp2)
	assertAppRepoCall(t, func(testee *AppRepo, c *mock.ClientMock) (err error) {
		c.MockResources = res
		retrieved, err := testee.GetAll(context.Background(), testApp.Namespace)
		if err == nil {
			require.Equal(t, expectedApps, retrieved, "get")
			require.Equal(t, expectedCalls, c.Calls, "client calls")
		}
		return
	})
}

func TestAppRepoDelete(t *testing.T) {
	kind := strings.ToLower(CrdKind) + "." + CrdAPIGroup
	expectedCalls := []string{
		fmt.Sprintf("delete myns/ [%s/%s]", kind, testApp.Name),
	}
	assertAppRepoCall(t, func(testee *AppRepo, c *mock.ClientMock) (err error) {
		if err = testee.Delete(context.Background(), testApp); err == nil {
			require.Equal(t, expectedCalls, c.Calls, "client calls")
		}
		return
	})
}
