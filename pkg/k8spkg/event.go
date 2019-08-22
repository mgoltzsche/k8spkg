package k8spkg

import (
	"strconv"
	"time"
)

const (
	timeLayout = "2006-01-02T15:04:05Z"
)

type Event struct {
	Type           string            `json:"type"`
	InvolvedObject InvolvedObjectRef `json:"involvedObject"`
	Count          int               `json:"count`
	Message        string            `json:"message"`
	Reason         string            `json:"reason"`
	Source         EventSource       `json:"source"`
	LastTimestamp  JSONTime          `json:"lastTimestamp"`
}

type JSONTime time.Time

func (t *JSONTime) UnmarshalJSON(v []byte) (err error) {
	s := string(v)
	s, err = strconv.Unquote(s)
	if err != nil {
		return
	}
	p, err := time.Parse(timeLayout, s)
	if err == nil {
		*t = JSONTime(p)
	}
	return
}

type InvolvedObjectRef struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Namespace  string `json:"namespace"`
	Name       string `json:"name"`
	Uid        string `json:"uid"`
}

type EventSource struct {
	Component string `json:"component"`
	Host      string `json:"host"`
}
