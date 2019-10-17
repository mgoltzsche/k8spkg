package resource

import (
	"sync"
)

func WatchEventUnion(ch []<-chan ResourceEvent) <-chan ResourceEvent {
	wg := sync.WaitGroup{}
	r := make(chan ResourceEvent)
	wg.Add(len(ch))
	for _, c := range ch {
		go func(c <-chan ResourceEvent) {
			for evt := range c {
				r <- evt
			}
			wg.Done()
		}(c)
	}
	go func() {
		wg.Wait()
		close(r)
	}()
	return r
}
