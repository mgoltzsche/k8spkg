package status

import (
	"fmt"
	"testing"
	"time"

	"github.com/mgoltzsche/k8spkg/pkg/resource"
	"github.com/stretchr/testify/require"
)

func TestTracker(t *testing.T) {
	n := 5
	obj := make([]resource.K8sResourceRef, n)
	names := make([]string, n)
	for i := 0; i < n; i++ {
		name := fmt.Sprintf("deployment-%d", i)
		obj[i] = resource.ResourceRef("apps/v1", "Deployment", "myns", name)
		names[i] = name
	}

	evts, receive := testee(obj)
	for _, o := range obj {
		go mockStatusEvent(o, false, evts)
		requireEvent(t, receive, "change "+o.Name()+" false")
	}
	for _, o := range obj[:len(obj)-1] {
		mockStatusEvent(o, true, evts)
		requireEvent(t, receive, "change "+o.Name()+" true")
	}
	mockStatusEvent(obj[len(obj)-1], true, evts)
	requireEvent(t, receive, "change "+obj[len(obj)-1].Name()+" true")
	requireEvent(t, receive, "ready true")
	mockStatusEvent(obj[len(obj)-1], false, evts)
	requireEvent(t, receive, "change "+obj[len(obj)-1].Name()+" false")
	close(evts)
	requireEvent(t, receive, "result false 1")
	requireEvent(t, receive, "closed")

	// test success
	evts, receive = testee(obj[:1])
	mockStatusEvent(obj[0], true, evts)
	requireEvent(t, receive, "change "+obj[0].Name()+" true")
	requireEvent(t, receive, "ready true")
	close(evts)
	requireEvent(t, receive, "result true 0")
	requireEvent(t, receive, "closed")
}

func testee(obj resource.K8sResourceRefList) (chan<- ResourceStatusEvent, <-chan string) {
	evts := make(chan ResourceStatusEvent)
	testee := NewTracker(obj, evts)
	ready := testee.Ready()
	changes := testee.Changes()
	result := testee.Result()
	receive := make(chan string)
	go testee.Run()
	go func() {
		for {
			if changes == nil && ready == nil && result == nil {
				break
			}
			select {
			case change, ok := <-changes:
				if !ok {
					changes = nil
					continue
				}
				if change.Err == nil {
					receive <- fmt.Sprintf("change %s %v", change.Resource.Name(), change.Status.Status)
				} else {
					receive <- fmt.Sprintf("change err %s", change.Err)
				}
			case rdy, ok := <-ready:
				if !ok {
					ready = nil
					continue
				}
				receive <- fmt.Sprintf("ready %v", rdy)
			case summary, ok := <-result:
				if !ok {
					result = nil
					continue
				}
				notReady := 0
				for _, res := range summary.Resources {
					if !res.Status.Status {
						notReady++
					}
				}
				receive <- fmt.Sprintf("result %v %d", summary.Ready, notReady)
			}
		}
		close(receive)
	}()
	return evts, receive
}

func requireEvent(t *testing.T, ch <-chan string, expected string) {
	select {
	case evt, ok := <-ch:
		if !ok {
			evt = "closed"
		}
		require.Equal(t, expected, evt, "received event")
	case <-time.After(time.Second):
		require.Failf(t, "timeout", "waiting for event %q", expected)
	}
}

func mockStatusEvent(o resource.K8sResourceRef, status bool, ch chan<- ResourceStatusEvent) {
	ch <- ResourceStatusEvent{ResourceStatus{o, ConditionStatus{Status: status}}, nil}
}
