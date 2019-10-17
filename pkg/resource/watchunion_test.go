package resource

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestWatchEventUnion(t *testing.T) {
	ch1 := make(chan ResourceEvent)
	ch2 := make(chan ResourceEvent)
	ch := WatchEventUnion([]<-chan ResourceEvent{ch1, ch2})
	for i, c := range []chan ResourceEvent{ch1, ch2} {
		go func(i int, c chan ResourceEvent) {
			c <- ResourceEvent{nil, fmt.Errorf("evt%d", i*2)}
			c <- ResourceEvent{nil, fmt.Errorf("evt%d", i*2+1)}
			close(c)
		}(i, c)
	}
	expectedEvts := map[string]bool{"evt0": true, "evt1": true, "evt2": true, "evt3": true}
	actualEvts := map[string]bool{}
	done := make(chan bool, 1)
	go func() {
		for evt := range ch {
			actualEvts[evt.Error.Error()] = true
			time.Sleep(10)
		}
		done <- true
		close(done)
	}()
	select {
	case <-done:
		require.EqualValues(t, expectedEvts, actualEvts)
	case <-time.After(time.Second):
		t.Error("timed out")
	}
}
