package cron

import (
	"sync"
	"time"
)

// tsrecVirtualTime is a concurrency-safe notion of time. It is normally frozen at a fixed instant; it
// can be set flowing, in which case it runs from that instant onwards, 1:1 with the wall clock.
type tsrecVirtualTime struct {
	mu      sync.Mutex
	flowing bool
	// frozen is the current instant while not flowing.
	frozen time.Time
	// realBase and virtualBase anchor the flow: while flowing, the current instant is derived as
	// virtualBase + (wall-clock now - realBase).
	realBase    time.Time
	virtualBase time.Time
}

func newTSRECVirtualTime(start time.Time) *tsrecVirtualTime {
	return &tsrecVirtualTime{frozen: start}
}

// now returns the current virtual instant.
func (v *tsrecVirtualTime) now() time.Time {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.nowLocked()
}

func (v *tsrecVirtualTime) nowLocked() time.Time {
	if v.flowing {
		return v.virtualBase.Add(time.Since(v.realBase))
	}
	return v.frozen
}

// advance moves the frozen time forward to t; it never moves backwards. While flowing it is a
// no-op: a flowing time has already reached t when its observer finds t due.
func (v *tsrecVirtualTime) advance(t time.Time) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if !v.flowing && v.frozen.Before(t) {
		v.frozen = t
	}
}

// flow sets the time flowing from its current instant onwards. It is a no-op if already flowing.
func (v *tsrecVirtualTime) flow() {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.flowing {
		return
	}
	v.virtualBase = v.frozen
	v.realBase = time.Now()
	v.flowing = true
}

// freeze pins the time at its current instant.
func (v *tsrecVirtualTime) freeze() {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.frozen = v.nowLocked()
	v.flowing = false
}
