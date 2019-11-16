package k8spkg

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/mgoltzsche/k8spkg/pkg/client"
	"github.com/mgoltzsche/k8spkg/pkg/resource"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Event struct {
	Type              string
	ObservedObject    resource.K8sResourceRef
	InvolvedObject    resource.K8sResourceRef
	InvolvedFieldPath string
	Message           string
	Reason            string
}

func Events(ctx context.Context, forRes resource.K8sResourceRefList, c client.K8sClient) <-chan Event {
	msgMap := map[string]bool{}
	knownNames := map[string]bool{}
	for _, res := range forRes {
		knownNames[res.ID()] = true
		knownNames[res.Name()] = true
	}
	ch := make(chan Event)
	wg := &sync.WaitGroup{}
	for _, byNs := range forRes.GroupByNamespace() {
		if byNs.Key != "" {
			evts := c.Watch(ctx, "Event", byNs.Key, nil, true)
			wg.Add(1)
			go func() {
				for evt := range evts {
					if evt.Error != nil {
						if ctx.Err() == nil {
							logrus.Errorf("watch events: %s", evt.Error)
						}
						continue
					}
					raw := evt.Resource.Raw()
					evtType, _, _ := unstructured.NestedString(raw, "type")
					reason, _, _ := unstructured.NestedString(raw, "reason")
					message, _, _ := unstructured.NestedString(raw, "message")
					rawInvolved, _, _ := unstructured.NestedMap(raw, "involvedObject")
					resApiVersion, _, _ := unstructured.NestedString(rawInvolved, "apiVersion")
					resKind, _, _ := unstructured.NestedString(rawInvolved, "kind")
					resName, _, _ := unstructured.NestedString(rawInvolved, "name")
					resNamespace, _, _ := unstructured.NestedString(rawInvolved, "namespace")
					resFieldPath, _, _ := unstructured.NestedString(rawInvolved, "fieldPath")
					involved := resource.ResourceRef(resApiVersion, resKind, resNamespace, resName)
					contained := knownNames[involved.ID()]
					if !contained && (resKind == "Pod" || resKind == "ReplicaSet" || resKind == "StatefulSet") {
						// include daemonset's or deployment's pod events
						if l := strings.Split(involved.Name(), "-"); len(l) > 2 {
							if contained = knownNames[strings.Join(l[:len(l)-1], "-")]; !contained {
								contained = knownNames[strings.Join(l[:len(l)-2], "-")]
							}
						}
					}
					if contained {
						// emit unique event
						msgKey := fmt.Sprintf("%s/%s: %s: %s: %s", involved.QualifiedKind(), resName, resFieldPath, reason, message)
						if !msgMap[msgKey] {
							msgMap[msgKey] = true
							ch <- Event{
								Type:              evtType,
								Reason:            reason,
								Message:           message,
								InvolvedObject:    involved,
								InvolvedFieldPath: resFieldPath,
							}
						}
					}
				}
				wg.Done()
			}()
		}
	}
	go func() {
		wg.Wait()
		close(ch)
	}()
	return ch
}

func containsNamePrefix(names map[string]bool, name string) bool {
	l := strings.Split(name, "-")
	if len(l) > 2 {
		return names[strings.Join(l[:len(l)-2], "-")]
	}
	return false
}
