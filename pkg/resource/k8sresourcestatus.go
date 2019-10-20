package resource

//type ResourceStatus map[string]interface{}

func (o *K8sResource) Conditions() []*K8sResourceCondition {
	return o.conditions
}

func (o *K8sResource) RolloutStatus(desiredField, readyField string) (desired, ready int) {
	status := asMap(o.raw["status"])
	desired, _ = status[desiredField].(int)
	ready, _ = status[readyField].(int)
	return
}
