package cron

import (
	"sync"
	"time"
)

// tsrecLoopState manages the lifecycle of a goroutine running a loop in the background: starting
// it, nudging it to re-inspect the state it derives its behaviour from, and asking it to exit or
// waiting for it to exit on its own. The goroutine waits for its next signal with await. It is
// concurrency-safe and its zero value is ready to use; the loop body itself is provided by the
// owner.
type tsrecLoopState struct {
	mu sync.Mutex
	// active is true from begin until the goroutine calls end.
	active bool
	// stopping is true once interrupt has closed stop, until the next begin.
	stopping bool
	// stop is closed to ask the goroutine to exit; done is closed by end once it has.
	stop chan struct{}
	done chan struct{}
	// wake (buffered) nudges the goroutine to re-inspect state.
	wake chan struct{}
}

// begin marks the loop active, setting up fresh signalling channels for the new goroutine. It
// reports whether it succeeded: false means a goroutine is already active.
func (l *tsrecLoopState) begin() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.active {
		return false
	}
	l.active = true
	l.stopping = false
	l.stop = make(chan struct{})
	l.done = make(chan struct{})
	l.wake = make(chan struct{}, 1)
	return true
}

// end marks the loop inactive and releases whoever is waiting for the goroutine to exit. The
// goroutine must call it exactly once, right before returning.
func (l *tsrecLoopState) end() {
	l.mu.Lock()
	l.active = false
	done := l.done
	l.mu.Unlock()
	close(done)
}

// await waits for the next event the goroutine should react to: a nudge, the expiry of the
// wall-clock timer (due may be nil when there is no deadline) or an interrupt. It reports whether
// the goroutine should keep running: false means interrupt asked it to exit. Only the active
// goroutine may call it.
func (l *tsrecLoopState) await(due <-chan time.Time) bool {
	l.mu.Lock()
	stop, wake := l.stop, l.wake
	l.mu.Unlock()
	select {
	case <-stop:
		return false
	case <-wake:
		return true
	case <-due:
		return true
	}
}

// nudge asks the active goroutine, if any, to re-inspect state. The signal is dropped when one is
// already pending: the goroutine re-derives everything it needs on each wake-up.
func (l *tsrecLoopState) nudge() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.active {
		return
	}
	select {
	case l.wake <- struct{}{}:
	default:
	}
}

// interrupt asks the active goroutine, if any, to exit and waits until it has.
func (l *tsrecLoopState) interrupt() {
	l.mu.Lock()
	if !l.active {
		l.mu.Unlock()
		return
	}
	if !l.stopping {
		l.stopping = true
		close(l.stop)
	}
	done := l.done
	l.mu.Unlock()
	<-done
}

// awaitEnd blocks until no goroutine is active, without asking it to exit.
func (l *tsrecLoopState) awaitEnd() {
	for {
		l.mu.Lock()
		if !l.active {
			l.mu.Unlock()
			return
		}
		done := l.done
		l.mu.Unlock()
		<-done
	}
}
