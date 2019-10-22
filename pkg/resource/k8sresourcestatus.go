package resource

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

//type ResourceStatus map[string]interface{}

func (o *K8sResource) Conditions() []*K8sResourceCondition {
	return o.conditions
}

func (o *K8sResource) RolloutStatus(desiredField, readyField string) (desired, ready int64) {
	desired, _, _ = unstructured.NestedInt64(o.raw, "status", desiredField)
	ready, _, _ = unstructured.NestedInt64(o.raw, "status", readyField)
	return
}

type ContainerStatus struct {
	Name         string
	Ready        bool
	ExitCode     int
	RestartCount int64
}

func (o *K8sResource) ContainerStatuses() (cs []ContainerStatus) {
	rawStats, _, _ := unstructured.NestedSlice(o.raw, "status", "containerStatuses")
	cs = make([]ContainerStatus, len(rawStats))

	for i, rawStat := range rawStats {
		statMap := asMap(rawStat)
		exitCode, _, _ := unstructured.NestedInt64(statMap, "lastState", "terminated", "exitCode")
		ready, _, _ := unstructured.NestedBool(statMap, "ready")
		restartCount, _, _ := unstructured.NestedInt64(statMap, "restartCount")
		cs[i] = ContainerStatus{
			Name:         asString(statMap["name"]),
			Ready:        ready,
			ExitCode:     int(exitCode),
			RestartCount: restartCount,
		}
	}
	return
}
