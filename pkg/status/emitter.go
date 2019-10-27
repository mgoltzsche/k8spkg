package status

import (
	"github.com/mgoltzsche/k8spkg/pkg/resource"
)

type ResourceStatus struct {
	Resource resource.K8sResourceRef
	Status   ConditionStatus
}

type ResourceStatusEvent struct {
	ResourceStatus
	Err error
}

func Emitter(ch <-chan resource.ResourceEvent, kindConditions map[string]Condition) <-chan ResourceStatusEvent {
	r := make(chan ResourceStatusEvent)
	go func() {
		statusMap := map[string]*ConditionStatus{}
		for evt := range ch {
			if evt.Error == nil {
				key := evt.Resource.ID()
				oldStatus := statusMap[key]
				cond := condition(evt.Resource.Kind(), kindConditions)
				if status := cond.Status(evt.Resource); !status.Equal(oldStatus) {
					statusMap[key] = &status
					r <- ResourceStatusEvent{ResourceStatus{
						Resource: evt.Resource,
						Status:   status,
					}, nil}
				}
			} else {
				r <- ResourceStatusEvent{Err: evt.Error}
			}
		}
		close(r)
	}()
	return r
}

func condition(kind string, kindConditions map[string]Condition) (c Condition) {
	if c = kindConditions[kind]; c == nil {
		c = condGeneric
	}
	return
}
