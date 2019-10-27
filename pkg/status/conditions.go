package status

import (
	"fmt"
	"strings"

	"github.com/mgoltzsche/k8spkg/pkg/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var (
	condGeneric       = genericCondition("genericCondition")
	RolloutConditions = map[string]Condition{
		"Deployment":  DeploymentRolloutCondition("available"),
		"Pod":         NewCondition("ready"),
		"Job":         NewCondition("ready"),
		"Certificate": NewCondition("ready"),
		"DaemonSet":   DaemonSetRolloutCondition("daemonset-rollout-condition"),
		"APIService":  NewCondition("available"),
	}
)

func NewCondition(condition string) Condition {
	return conditionType(strings.ToLower(condition))
}

type Condition interface {
	Status(o *resource.K8sResource) (status ConditionStatus)
}

type ConditionStatus struct {
	Status      bool
	Description string
}

func (c *ConditionStatus) Equal(s *ConditionStatus) bool {
	return s != nil && c.Status == s.Status && c.Description == s.Description
}

type conditionType string

func (c conditionType) Status(o *resource.K8sResource) (r ConditionStatus) {
	for _, cond := range o.Conditions() {
		if strings.ToLower(cond.Type) == string(c) {
			msg := cond.Reason
			if msg == "" {
				msg = cond.Type
			}
			if cond.Message != "" {
				msg += ": " + cond.Message
			}
			r.Description = msg
			r.Status = cond.Status
			return
		}
	}
	r.Description = fmt.Sprintf("condition %s not present", c)
	return
}

type genericCondition string

func (c genericCondition) Status(o *resource.K8sResource) (r ConditionStatus) {
	conditionsMet := make([]string, 0, len(o.Conditions()))
	for _, cond := range o.Conditions() {
		if !cond.Status {
			msg := cond.Type + ": " + cond.Reason
			if cond.Message != "" {
				msg += ": " + cond.Message
			}
			r.Description = msg
			r.Status = cond.Status
			return
		} else {
			conditionsMet = append(conditionsMet, strings.ToLower(cond.Type))
		}
	}
	r.Status = true
	if len(o.Conditions()) == 0 {
		r.Description = "is present"
	} else {
		r.Description = strings.Join(conditionsMet, ", ")
	}
	return
}

type DeploymentRolloutCondition conditionType

func (c DeploymentRolloutCondition) Status(o *resource.K8sResource) (r ConditionStatus) {
	s := conditionType(c).Status(o)
	replicas, _, _ := unstructured.NestedFloat64(o.Raw(), "spec", "replicas")
	scheduledReplicas, _, _ := unstructured.NestedFloat64(o.Raw(), "status", "replicas")
	readyReplicas, _, _ := unstructured.NestedFloat64(o.Raw(), "status", "readyReplicas")
	updatedReplicas, _, _ := unstructured.NestedFloat64(o.Raw(), "status", "updatedReplicas")
	generation, _, _ := unstructured.NestedFloat64(o.Raw(), "metadata", "generation")
	generationObserved, _, _ := unstructured.NestedFloat64(o.Raw(), "status", "observedGeneration")
	generationUpToDate := generation == generationObserved
	if !generationUpToDate {
		updatedReplicas = 0
	}
	status := generationUpToDate &&
		updatedReplicas == replicas &&
		readyReplicas == scheduledReplicas &&
		readyReplicas == replicas
	r.Status = s.Status && status
	suffix := "ready"
	if !r.Status {
		suffix = "updated"
		if status && !s.Status {
			suffix += ", " + s.Description
		}
	}
	r.Description = fmt.Sprintf("%.0f/%.0f %s", updatedReplicas, replicas, suffix)
	return
}

type DaemonSetRolloutCondition string

func (c DaemonSetRolloutCondition) Status(o *resource.K8sResource) (r ConditionStatus) {
	if r = condGeneric.Status(o); r.Status {
		misscheduled, _, _ := unstructured.NestedFloat64(o.Raw(), "status", "numberMisscheduled")
		desired, _, _ := unstructured.NestedFloat64(o.Raw(), "status", "desiredNumberScheduled")
		ready, _, _ := unstructured.NestedFloat64(o.Raw(), "status", "numberReady")
		generation, _, _ := unstructured.NestedFloat64(o.Raw(), "metadata", "generation")
		generationObserved, _, _ := unstructured.NestedFloat64(o.Raw(), "status", "observedGeneration")
		r.Status = desired == ready &&
			generation == generationObserved &&
			int64(misscheduled) == 0
		r.Description = fmt.Sprintf("%.0f/%.0f ready", ready, desired)
	}
	return
}
