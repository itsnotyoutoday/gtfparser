package gtfparser

import (
	"sync"
	"time"
)

// Runner owns an Interp in a single goroutine (an actor). An Interp is not
// safe for concurrent use; every other goroutine (sockets, the terminal,
// timers, other users' sessions) talks to it through Do/Post. Runner also
// drives /repeat and /quote timers.
type Runner struct {
	in   *Interp
	ch   chan func(*Interp)
	done chan struct{}
	once sync.Once

	mu     sync.RWMutex // guards closed and every send on ch against close(ch)
	closed bool
}

// NewRunner starts the goroutine that owns in. Do not touch in directly
// afterwards, except Interrupt, which is safe from anywhere.
func NewRunner(in *Interp) *Runner {
	r := &Runner{in: in, ch: make(chan func(*Interp), 256), done: make(chan struct{})}
	go r.loop()
	return r
}

func (r *Runner) loop() {
	defer close(r.done)
	timer := time.NewTimer(time.Hour)
	defer timer.Stop()
	for {
		if next, ok := r.in.NextDue(); ok {
			d := time.Until(next)
			if d < 0 {
				d = 0
			}
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(d)
		}
		select {
		case fn, ok := <-r.ch:
			if !ok {
				return
			}
			fn(r.in)
		case <-timer.C:
			r.in.RunDue()
		}
	}
}

// Post queues fn to run on the interpreter's goroutine and returns at once.
// After Close it does nothing (the work is dropped), so a late caller, such as
// a socket reader racing teardown, never sends on a closed channel.
func (r *Runner) Post(fn func(*Interp)) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.closed {
		return
	}
	r.ch <- fn
}

// Do runs fn on the interpreter's goroutine and waits for it. After Close it
// returns at once without running fn.
func (r *Runner) Do(fn func(*Interp)) {
	d := make(chan struct{})
	r.mu.RLock()
	if r.closed {
		r.mu.RUnlock()
		return
	}
	r.ch <- func(in *Interp) { fn(in); close(d) }
	r.mu.RUnlock()
	<-d
}

// Interrupt stops a runaway script (safe from any goroutine).
func (r *Runner) Interrupt() { r.in.Interrupt() }

// Close stops the goroutine after queued work has run.
func (r *Runner) Close() {
	r.once.Do(func() {
		r.mu.Lock()
		r.closed = true
		close(r.ch)
		r.mu.Unlock()
	})
	<-r.done
}
