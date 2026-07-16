package cron

import (
	"sync"
	"time"
)

// tsrecSchedulerTimer is a concurrency-safe rendezvous between the cron scheduler and whoever fires its
// timers manually. The scheduler side arms a timer and sleeps on the returned channel; the firing
// side observes what the scheduler is waiting for and fires it. Because the scheduler may disarm
// or re-arm the timer at any moment, firing is a compare-and-fire: it succeeds only if the timer
// is still armed for the expected activation. The zero value is ready to use.
type tsrecSchedulerTimer struct {
	mu sync.Mutex
	// armed is true while the scheduler is sleeping, waiting for the timer to fire.
	armed bool
	// at is the scheduled activation time; zero for a nop timer, which never fires.
	at    time.Time
	timer chan struct{}
}

// arm records that the scheduler went to sleep waiting for an activation at t, and returns the
// timer channel it sleeps on.
func (s *tsrecSchedulerTimer) arm(t time.Time) <-chan struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.armed = true
	s.at = t
	s.timer = make(chan struct{}, 1)
	return s.timer
}

// armNop records that the scheduler went to sleep with no scheduled activation, and returns a
// timer channel that never fires.
func (s *tsrecSchedulerTimer) armNop() <-chan struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.armed = true
	s.at = time.Time{}
	s.timer = make(chan struct{})
	return s.timer
}

// disarm records that the scheduler woke up for a reason other than the timer firing.
func (s *tsrecSchedulerTimer) disarm() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.armed = false
}

// state returns whether the scheduler is currently sleeping on an armed timer, and the activation
// time it is waiting for (zero for a nop timer). The result is a snapshot: the scheduler may
// disarm or re-arm concurrently, which fireIfArmedAt detects.
func (s *tsrecSchedulerTimer) state() (at time.Time, armed bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.at, s.armed
}

// fireIfArmedAt fires the timer, waking the scheduler, provided it is still armed for an
// activation at t. It reports whether it fired: false means the scheduler disarmed or re-armed
// the timer concurrently and the caller should re-inspect the new state.
func (s *tsrecSchedulerTimer) fireIfArmedAt(t time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.armed || s.at.IsZero() || !s.at.Equal(t) {
		return false
	}
	s.armed = false
	s.timer <- struct{}{}
	close(s.timer)
	return true
}
