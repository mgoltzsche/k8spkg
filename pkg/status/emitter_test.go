package status

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/mgoltzsche/k8spkg/pkg/resource"
	"github.com/stretchr/testify/require"
)

func TestEmitter(t *testing.T) {
	b, err := ioutil.ReadFile("../client/mock/watch.json")
	require.NoError(t, err)
	evts := make(chan resource.ResourceEvent)
	changes := Emitter(evts, RolloutConditions)
	go func() {
		for evt := range resource.FromJsonStream(bytes.NewReader(b)) {
			evts <- evt
		}
		evts <- resource.ResourceEvent{Error: fmt.Errorf("mock error")}
		close(evts)
	}()
	received := []string{}
	for c := range changes {
		if c.Err == nil {
			received = append(received, fmt.Sprintf("%s: %v %s", c.Resource.Name(), c.Status.Status, c.Status.Description))
		} else {
			received = append(received, fmt.Sprintf("err %s", c.Err))
		}
	}
	expected := []string{
		"mydeployment: false 0/1 updated",
		"mydeployment: false 1/1 updated",
		"mydeployment: true 1/1 ready",
		"mydeployment: false 1/1 updated",
		"mydeployment: true 1/1 ready",
		"mydeployment: false 1/1 updated",
		"mydeployment: true 1/1 ready",
		"somedeployment-pod-x: true Ready: Pod is ready",
		"otherdeployment: true 1/1 ready",
	}
	expected = append(expected, "err mock error")
	require.Equal(t, expected, received, "received status change events")
}

func mockResourceEvent(o resource.K8sResourceRef, status bool, descr string, ch chan<- resource.ResourceEvent) {
	ch <- resource.ResourceEvent{}
}
