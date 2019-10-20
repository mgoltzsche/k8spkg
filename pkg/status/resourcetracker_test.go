package status

import (
	"os"
	"testing"

	"github.com/mgoltzsche/k8spkg/pkg/resource"
	"github.com/stretchr/testify/require"
)

func TestResourceTracker(t *testing.T) {
	f, err := os.Open("../client/mock/watch.json")
	require.NoError(t, err)
	defer f.Close()
	var obj resource.K8sResourceList
	for evt := range resource.FromJsonStream(f) {
		require.NoError(t, evt.Error)
		obj = append(obj, evt.Resource)
	}
	testee := NewResourceTracker(obj, RolloutConditions)
	hasReady := false
	hasNotReady := false
	hasChanged := false
	hasUnchanged := false
	status, changed := testee.Update(obj[0])
	require.True(t, changed, "first update should be considered a change (%s)", obj[0].ID())
	_, changed = testee.Update(obj[0])
	require.False(t, changed, "update should not mark a change")
	for _, o := range obj[:len(obj)-1] {
		status, changed = testee.Update(o)
		if status.Status {
			hasReady = true
		} else {
			hasNotReady = true
		}
		if changed {
			hasChanged = true
		} else {
			hasUnchanged = true
		}
	}
	require.True(t, hasReady, "has positive status")
	require.True(t, hasNotReady, "has negative status")
	require.True(t, hasChanged, "has changed")
	require.True(t, hasUnchanged, "has unchanged")
	require.False(t, testee.Found(), "all seen")
	require.False(t, testee.Ready(), "all ready")
	lastObj := obj[len(obj)-1]
	testee.Update(lastObj)
	require.True(t, testee.Found(), "all seen")
	require.False(t, testee.Ready(), "all ready")
	lastObj.Conditions()[0].Status = true
	_, changed = testee.Update(lastObj)
	require.True(t, changed, "update with changed status should mark change")
	require.True(t, testee.Ready(), "all ready")
	require.True(t, testee.Found(), "all seen")
}
