package status

import (
	"github.com/mgoltzsche/k8spkg/pkg/resource"
)

var (
	DefaultCondition = ConditionStatus{false, "awaiting status update"}
)

type ResourceTracker struct {
	resources      map[string]*TrackedResource
	order          []string
	kindConditions map[string]Condition
	required       int
	ready          int
	found          int
}

type TrackedResource struct {
	Resource  *resource.K8sResource
	condition Condition
	Status    ConditionStatus
	required  bool
}

func (r *TrackedResource) update(o *resource.K8sResource) (status ConditionStatus, changed bool) {
	r.Resource = o
	if s := r.condition.Status(o); !s.Equal(&r.Status) {
		r.Status = s
		changed = true
	}
	status = r.Status
	return
}

func NewResourceTracker(obj resource.K8sResourceList, kindConditions map[string]Condition) *ResourceTracker {
	m := map[string]*TrackedResource{}
	order := make([]string, 0, len(obj))
	for _, res := range obj {
		key := res.ID()
		cond := kindConditions[res.Kind()]
		if cond == nil {
			cond = condGeneric
		}
		if m[key] == nil {
			order = append(order, key)
		}
		c := condition(res.Kind(), kindConditions)
		m[key] = &TrackedResource{res, c, DefaultCondition, true}
	}
	return &ResourceTracker{m, order, kindConditions, len(order), 0, 0}
}

func condition(kind string, kindConditions map[string]Condition) (c Condition) {
	if c = kindConditions[kind]; c == nil {
		c = condGeneric
	}
	return
}

// Update updates the resource and returns its status as well as a flag indicating a change caused by the update
func (t *ResourceTracker) Update(o *resource.K8sResource) (status ConditionStatus, changed bool) {
	if res := t.resources[o.ID()]; res != nil {
		if res.required && res.Status == DefaultCondition {
			t.found++
		}
		oldStatus := res.Status.Status
		status, changed = res.update(o)
		if res.required && status.Status != oldStatus { // status changed
			if status.Status {
				t.ready++
			} else {
				t.ready--
			}
		}
	} else {
		// TODO: filter pods that are owned by initially registered resources
		//   -> match (pods) by label
		res := &TrackedResource{o, condition(o.Kind(), t.kindConditions), DefaultCondition, false}
		t.resources[o.ID()] = res
		status, changed = res.update(o)
	}
	return
}

func (t *ResourceTracker) Ready() bool {
	return t.ready == t.required
}

func (t *ResourceTracker) Found() bool {
	return t.found == t.required
}

func (t *ResourceTracker) Status() (status []*TrackedResource) {
	status = make([]*TrackedResource, len(t.order))
	for i, key := range t.order {
		status[i] = t.resources[key]
	}
	return
}
