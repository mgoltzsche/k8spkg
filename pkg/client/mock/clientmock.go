package mock

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"strings"
	"sync"

	"github.com/mgoltzsche/k8spkg/pkg/client"
	"github.com/mgoltzsche/k8spkg/pkg/resource"
)

func MockDataFile(file string) (mockOut []byte) {
	mockOut, err := ioutil.ReadFile(file)
	if err != nil {
		panic(err)
	}
	return
}

func MockResourceList(file string) (l resource.K8sResourceList) {
	l, err := resource.FromYaml(bytes.NewReader(MockDataFile(file)))
	if err != nil {
		panic(err)
	}
	return
}

type ClientMock struct {
	Calls           []string
	MockErr         error
	Applied         resource.K8sResourceList
	MockResource    *resource.K8sResource
	MockResources   resource.K8sResourceList
	MockWatchEvents []resource.ResourceEvent
	MockTypes       []*client.APIResourceType
	lock            sync.Mutex
}

func NewClientMock() *ClientMock {
	return &ClientMock{}
}

func (c *ClientMock) call(f string, args ...interface{}) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.Calls = append(c.Calls, strings.TrimSpace(fmt.Sprintf(f, args...)))
}

func requireContext(ctx context.Context) {
	if ctx == nil {
		panic("no context provided")
	}
}

func (c *ClientMock) Apply(ctx context.Context, namespace string, resources resource.K8sResourceList, prune bool, labels []string) (r resource.K8sResourceList, err error) {
	requireContext(ctx)
	c.call("apply %s/ %v %+v", namespace, prune, labels)
	c.Applied = resources
	return resources, c.MockErr
}
func (c *ClientMock) Delete(ctx context.Context, namespace string, resources resource.K8sResourceRefList) (err error) {
	requireContext(ctx)
	c.call("delete %s/ %+v", namespace, resources.Names())
	return c.MockErr
}
func (c *ClientMock) AwaitDeletion(ctx context.Context, namespace string, resources resource.K8sResourceRefList) (err error) {
	requireContext(ctx)
	c.call("awaitdeletion %s/ %+v", namespace, resources.Names())
	return c.MockErr
}
func (c *ClientMock) Get(ctx context.Context, kinds []string, namespace string, labels []string) (resource.K8sResourceList, error) {
	requireContext(ctx)
	c.call("get %s/ %s %+v", namespace, strings.Join(kinds, ","), labels)
	return c.MockResources, c.MockErr
}
func (c *ClientMock) GetResource(ctx context.Context, kind, namespace, name string) (*resource.K8sResource, error) {
	requireContext(ctx)
	c.call("getresource %s/ %s %s", namespace, kind, name)
	if c.MockResource == nil {
		return nil, fmt.Errorf("mock cient: get resource: no mock resource specified")
	}
	return c.MockResource, c.MockErr
}
func (c *ClientMock) Watch(ctx context.Context, kind, namespace string, labels []string) <-chan resource.ResourceEvent {
	requireContext(ctx)
	c.call("watch %s/%s %+v", namespace, kind, labels)
	ch := make(chan resource.ResourceEvent)
	go func() {
		for _, evt := range c.MockWatchEvents {
			ch <- evt
		}
		close(ch)
	}()
	return ch
}
func (c *ClientMock) ResourceTypes(ctx context.Context) (types []*client.APIResourceType, err error) {
	requireContext(ctx)
	c.call("resourcetypes")
	return c.MockTypes, c.MockErr
}