package status

import (
	"fmt"
	"strings"

	"github.com/mgoltzsche/k8spkg/pkg/resource"
)

var (
	condGeneric       = genericCondition("genericCondition")
	RolloutConditions = map[string]Condition{
		"Deployment":  &RolloutCondition{"replicas", "readyReplicas"},
		"Pod":         NewCondition("ready"),
		"Job":         NewCondition("ready"),
		"Certificate": NewCondition("ready"),
		"DaemonSet":   &RolloutCondition{"desiredNumberScheduled", "numberReady"},
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
			r.Status = cond.Status
			msg := cond.Reason
			if msg == "" {
				msg = cond.Type
			}
			if cond.Message != "" {
				msg += ": " + cond.Message
			}
			r.Description = msg
			return
		}
	}
	r.Description = fmt.Sprintf("condition %q is not present", c)
	return
}

type genericCondition string

func (c genericCondition) Status(o *resource.K8sResource) (r ConditionStatus) {
	conditionsMet := make([]string, 0, len(o.Conditions()))
	for _, cond := range o.Conditions() {
		if !cond.Status {
			r.Status = cond.Status
			r.Description = cond.Type + ":" + cond.Reason + ": " + cond.Message
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

type RolloutCondition struct {
	DesiredField string
	ReadyField   string
}

func (c *RolloutCondition) Status(o *resource.K8sResource) (r ConditionStatus) {
	if r = condGeneric.Status(o); r.Status {
		desired, ready := o.RolloutStatus(c.DesiredField, c.ReadyField)
		r.Status = desired == ready // TODO: sufficient? consider updatedReplicas?
		r.Description = fmt.Sprintf("%d/%d ready", ready, desired)
	}
	return
}
