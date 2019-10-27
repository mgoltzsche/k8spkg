package status

import (
	"sync"

	"github.com/mgoltzsche/k8spkg/pkg/resource"
	"github.com/sirupsen/logrus"
)

var (
	DefaultStatus = ConditionStatus{false, "awaiting status update"}
)

type Tracker struct {
	status    <-chan ResourceStatusEvent
	resources map[string]*ResourceStatus
	order     []string
	required  int
	ready     int
	found     int
	readyCh   chan bool
	resultCh  chan StatusReport
	changeCh  chan ResourceStatusEvent
	lock      *sync.Mutex
}

type StatusReport struct {
	Ready     bool
	Resources []*ResourceStatus
}

func NewTracker(obj resource.K8sResourceRefList, status <-chan ResourceStatusEvent) (t *Tracker) {
	statusMap := map[string]*ResourceStatus{}
	order := make([]string, 0, len(obj))
	for _, res := range obj {
		key := res.ID()
		if statusMap[key] == nil {
			order = append(order, key)
		}
		statusMap[key] = &ResourceStatus{res, DefaultStatus}
	}
	return &Tracker{status, statusMap, order, len(order), 0, 0, nil, nil, nil, &sync.Mutex{}}
}

func (t *Tracker) Run() StatusReport {
	for evt := range t.status {
		t.update(evt)
	}
	if t.changeCh != nil {
		close(t.changeCh)
	}
	t.reportReady()
	t.reportResult()
	return t.statusReport()
}

func (t *Tracker) update(evt ResourceStatusEvent) {
	if evt.Err != nil {
		t.delegateEvent(evt)
		return
	}
	if res := t.resources[evt.Resource.ID()]; res != nil {
		if res.Status == DefaultStatus {
			t.found++
		}
		t.delegateEvent(evt)
		if evt.Status.Status != res.Status.Status { // status changed
			if evt.Status.Status {
				t.ready++
			} else {
				t.ready--
			}
			if t.isReady() {
				t.reportReady()
			}
		}
		res.Status = evt.Status
	}
	return
}

func (t *Tracker) Changes() <-chan ResourceStatusEvent {
	t.changeCh = make(chan ResourceStatusEvent)
	return t.changeCh
}

func (t *Tracker) delegateEvent(evt ResourceStatusEvent) {
	if t.changeCh != nil {
		t.changeCh <- evt
	} else if evt.Err != nil {
		logrus.Errorf("status tracker: %s", evt.Err)
	}
}

func (t *Tracker) isReady() bool {
	return t.ready == t.required
}

func (t *Tracker) Ready() <-chan bool {
	t.readyCh = make(chan bool, 1)
	return t.readyCh
}

func (t *Tracker) reportReady() {
	if t.readyCh == nil {
		return
	}
	t.readyCh <- t.isReady()
	close(t.readyCh)
	t.readyCh = nil
}

func (t *Tracker) Result() <-chan StatusReport {
	t.resultCh = make(chan StatusReport, 1)
	return t.resultCh
}

func (t *Tracker) reportResult() {
	if t.resultCh == nil {
		return
	}
	status := make([]*ResourceStatus, len(t.order))
	for i, key := range t.order {
		status[i] = t.resources[key]
	}
	t.resultCh <- t.statusReport()
	close(t.resultCh)
	t.resultCh = nil
}

func (t *Tracker) statusReport() StatusReport {
	status := make([]*ResourceStatus, len(t.order))
	for i, key := range t.order {
		status[i] = t.resources[key]
	}
	return StatusReport{t.isReady(), status}
}
