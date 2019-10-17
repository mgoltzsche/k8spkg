package status

import (
	"fmt"
	"strings"

	"github.com/mgoltzsche/k8spkg/pkg/resource"
)

var (
	RolloutConditions = map[string]Condition{
		"Deployment":  NewCondition("available"),
		"Pod":         NewCondition("ready"),
		"Job":         NewCondition("ready"),
		"Certificate": NewCondition("ready"),
		//"DaemonSet":  NewCondition("available"),
	}
	fallbackCondition = genericCondition("genericCondition")
)

func NewCondition(condition string) Condition {
	return conditionType(strings.ToLower(condition))
}

type Condition interface {
	Status(o *resource.K8sResource, status *ConditionStatus)
}

type ConditionStatus struct {
	Status      bool
	Description string
}

func (c *ConditionStatus) Equal(s *ConditionStatus) bool {
	return c.Status == s.Status && c.Description == s.Description
}

type conditionType string

func (c conditionType) Status(o *resource.K8sResource, r *ConditionStatus) {
	for _, cond := range o.Conditions() {
		if strings.ToLower(cond.Type) == string(c) {
			r.Status = cond.Status
			r.Description = cond.Reason + ": " + cond.Message
			return
		}
	}
	r.Description = fmt.Sprintf("condition %q is not present", c)
}

type genericCondition string

func (c genericCondition) Status(o *resource.K8sResource, r *ConditionStatus) {
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
}
