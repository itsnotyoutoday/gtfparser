package gtfparser

import (
	"sync"
	"sync/atomic"
	"testing"
)

// Post and Do racing Close must neither panic (send on closed channel) nor trip the race detector,
// and work queued before Close still runs.
func TestRunnerPostRacingClose(t *testing.T) {
	for i := 0; i < 50; i++ {
		r := NewRunner(New(&tHost{}))
		var ran atomic.Int64
		var wg sync.WaitGroup
		for g := 0; g < 8; g++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 200; j++ {
					r.Post(func(*Interp) { ran.Add(1) })
					r.Do(func(*Interp) { ran.Add(1) })
				}
			}()
		}
		r.Close()
		wg.Wait()
		r.Post(func(*Interp) { t.Error("work ran after Close") })
		r.Do(func(*Interp) { t.Error("Do ran after Close") })
		r.Close() // idempotent
		_ = ran.Load()
	}
}
