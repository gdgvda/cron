package cron

import (
	"sync"
	"sync/atomic"
	"time"
)

// TimerSkippingRealExecutionClock is a manual clock that skips the wait between scheduled
// activations, like TimerSkippingInstantExecutionClock, but lets jobs execute in real time while
// the clock flows along with them.
//
// While no job is running the clock is frozen and jumps straight to the next activation; while
// triggered jobs are executing it flows 1:1 with the wall clock, so that the jobs observe time
// passing and further activations fire as the flowing clock reaches them. AdvanceTo and AdvanceBy
// return as soon as the clock reaches the requested instant, without waiting for the jobs they
// started; WaitForIdle blocks until those jobs have completed and the clock is frozen again.
//
// Every method is safe for concurrent use. AdvanceTo, AdvanceBy and WaitForIdle are serialized
// against each other: each runs to completion before the next one begins.
type TimerSkippingRealExecutionClock struct {
	// driver serializes the manual operations, so that only one advance is ever in flight.
	driver sync.Mutex
	time   *virtualTime
	timer  *schedulerTimer
	// cycles counts the fired activations whose jobs have not yet completed.
	cycles atomic.Int64
	// loop manages the goroutine that runs runLoop during and after each advance.
	loop *loopState
}

func NewTimerSkippingRealExecutionClock(start time.Time) *TimerSkippingRealExecutionClock {
	return &TimerSkippingRealExecutionClock{
		time:  newVirtualTime(start),
		timer: &schedulerTimer{},
		loop:  &loopState{},
	}
}

func (c *TimerSkippingRealExecutionClock) Register(cron *Cron) []Option {
	return []Option{WithOnCycleCompleted(
		func() {
			c.cycles.Add(-1)
			c.loop.nudge()
		},
	)}
}

func (c *TimerSkippingRealExecutionClock) Now() time.Time {
	return c.time.now()
}

func (c *TimerSkippingRealExecutionClock) Timer(t time.Time) (<-chan struct{}, func()) {
	timer := c.timer.arm(t)
	c.loop.nudge()
	return timer, c.timer.disarm
}

func (c *TimerSkippingRealExecutionClock) NopTimer() (<-chan struct{}, func()) {
	timer := c.timer.armNop()
	c.loop.nudge()
	return timer, c.timer.disarm
}

// AdvanceBy moves the virtual clock forward by duration, firing every activation scheduled within
// the interval. Jobs may still be running when it returns. There is no timeout: advancing needs a
// scheduler sleeping on a timer, and none is before Cron.Start or after Cron.Stop, so advancing
// then blocks forever.
func (c *TimerSkippingRealExecutionClock) AdvanceBy(duration time.Duration) {
	c.driver.Lock()
	defer c.driver.Unlock()
	c.loop.interrupt()
	c.advance(c.time.now().Add(duration))
}

// AdvanceTo moves the virtual clock forward to target, firing every activation scheduled up to and
// including it. Jobs may still be running when it returns. There is no timeout: advancing needs a
// scheduler sleeping on a timer, and none is before Cron.Start or after Cron.Stop, so advancing
// then blocks forever.
func (c *TimerSkippingRealExecutionClock) AdvanceTo(target time.Time) {
	c.driver.Lock()
	defer c.driver.Unlock()
	c.loop.interrupt()
	c.advance(target)
}

// WaitForIdle blocks until every job started by fired activations has completed and the clock is
// frozen again. There is no timeout: it blocks forever if a job never returns, or if jobs outlast
// the interval to the next activation, so that the flowing clock keeps starting new ones.
func (c *TimerSkippingRealExecutionClock) WaitForIdle() {
	c.driver.Lock()
	defer c.driver.Unlock()
	// Waiting for the loop to end is enough: it only returns on its own once no cycle is running,
	// and the only other way out is interrupt, which every caller takes under driver.
	c.loop.awaitEnd()
}

// advance returns once the clock has reached target; the runLoop goroutine may outlive the call,
// keeping the clock flowing until every started job has completed. The caller must hold driver and
// have interrupted the previous loop.
func (c *TimerSkippingRealExecutionClock) advance(target time.Time) {
	c.loop.begin()
	reached := make(chan struct{})
	go c.runLoop(target, reached)
	<-reached
}

// runLoop moves the virtual clock to target and beyond, closing reached as soon as target is
// reached. It returns — freezing the clock where it stands — once target is reached and no job is
// running, or when the next advance interrupts it.
func (c *TimerSkippingRealExecutionClock) runLoop(target time.Time, reached chan<- struct{}) {
	defer func() {
		c.time.freeze()
		c.loop.end()
	}()

	targetReached := false
	reach := func() {
		if !targetReached {
			targetReached = true
			close(reached)
		}
	}

	for {
		at, armed := c.timer.state()

		// The scheduler is awake, processing its last wake-up: nothing can be concluded about the
		// remaining activations until it arms its next timer, which nudges the loop back here.
		if !armed {
			if !c.loop.await(nil) {
				return
			}
			continue
		}

		// The scheduler sleeps and no job is executing: the clock freezes and jumps over idle time.
		if c.cycles.Load() == 0 {
			c.time.freeze()
			if at.IsZero() || at.After(target) {
				// No activation fits the advance: jump to target and exit, the deferred freeze
				// pinning the clock until the next advance.
				c.time.advance(target)
				reach()
				return
			}
			// Jump straight to the armed activation and fire it; the jobs it starts, if any, set
			// the clock flowing from that very instant.
			c.time.advance(at)
			c.fire(at)
			if c.cycles.Load() > 0 {
				c.time.flow()
			}
			continue
		}

		// Jobs are executing: the clock flows with the wall clock so that they observe time passing.
		c.time.flow()
		now := c.time.now()
		if !now.Before(target) {
			reach()
		}
		if !at.IsZero() && !at.After(now) {
			c.fire(at)
			continue
		}

		// Nothing is due yet: sleep until the earliest pending instant — the armed activation
		// and/or target — or indefinitely when neither is pending. A nudge (timer re-armed, cycle
		// completed) cuts the sleep short, and the loop re-derives everything from fresh state.
		deadline := at
		if !targetReached && (deadline.IsZero() || target.Before(deadline)) {
			deadline = target
		}
		if deadline.IsZero() {
			if !c.loop.await(nil) {
				return
			}
			continue
		}
		wallTimer := time.NewTimer(deadline.Sub(now))
		alive := c.loop.await(wallTimer.C)
		wallTimer.Stop()
		if !alive {
			return
		}
	}
}

// fire fires the activation scheduled at at; the caller must have brought the virtual clock to at
// beforehand. The cycle is counted as running before the scheduler is woken, so that it can never
// be observed completing before it started.
func (c *TimerSkippingRealExecutionClock) fire(at time.Time) {
	c.cycles.Add(1)
	if !c.timer.fireIfArmedAt(at) {
		// The scheduler disarmed or re-armed its timer concurrently: no cycle started after all.
		c.cycles.Add(-1)
	}
}

// virtualTime is a concurrency-safe notion of time. It is normally frozen at a fixed instant; it
// can be set flowing, in which case it runs from that instant onwards, 1:1 with the wall clock.
type virtualTime struct {
	mu      sync.Mutex
	flowing bool
	frozen  time.Time
	// realBase and virtualBase anchor the flow: while flowing, the current instant is
	// virtualBase + (wall-clock now - realBase).
	realBase    time.Time
	virtualBase time.Time
}

func newVirtualTime(start time.Time) *virtualTime {
	return &virtualTime{frozen: start}
}

func (v *virtualTime) now() time.Time {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.nowLocked()
}

func (v *virtualTime) nowLocked() time.Time {
	if v.flowing {
		return v.virtualBase.Add(time.Since(v.realBase))
	}
	return v.frozen
}

// advance moves the frozen time forward to t; it never moves backwards. While flowing it is a
// no-op: a flowing time has already reached t when its observer finds t due.
func (v *virtualTime) advance(t time.Time) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if !v.flowing && v.frozen.Before(t) {
		v.frozen = t
	}
}

func (v *virtualTime) flow() {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.flowing {
		return
	}
	v.virtualBase = v.frozen
	v.realBase = time.Now()
	v.flowing = true
}

func (v *virtualTime) freeze() {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.frozen = v.nowLocked()
	v.flowing = false
}

// schedulerTimer is a concurrency-safe rendezvous between the cron scheduler and whoever fires its
// timers manually. The scheduler side arms a timer and sleeps on the returned channel; the firing
// side observes what the scheduler is waiting for and fires it. Because the scheduler may disarm
// or re-arm the timer at any moment, firing is a compare-and-fire: it succeeds only if the timer
// is still armed for the expected activation. The zero value is ready to use.
type schedulerTimer struct {
	mu sync.Mutex
	// armed is true while the scheduler is sleeping, waiting for the timer to fire.
	armed bool
	// at is the scheduled activation time; zero for a nop timer, which never fires.
	at    time.Time
	timer chan struct{}
}

func (s *schedulerTimer) arm(t time.Time) <-chan struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.armed = true
	s.at = t
	s.timer = make(chan struct{}, 1)
	return s.timer
}

func (s *schedulerTimer) armNop() <-chan struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.armed = true
	s.at = time.Time{}
	s.timer = make(chan struct{})
	return s.timer
}

// disarm records that the scheduler woke up for a reason other than the timer firing.
func (s *schedulerTimer) disarm() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.armed = false
}

// state returns the activation time the scheduler is sleeping on (zero for a nop timer) and
// whether it is sleeping at all. It is a snapshot: the scheduler may disarm or re-arm
// concurrently, which fireIfArmedAt detects.
func (s *schedulerTimer) state() (at time.Time, armed bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.at, s.armed
}

// fireIfArmedAt fires the timer, waking the scheduler, provided it is still armed for an
// activation at t. It reports whether it fired: false means the scheduler disarmed or re-armed
// the timer concurrently and the caller should re-inspect the new state.
func (s *schedulerTimer) fireIfArmedAt(t time.Time) bool {
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

// loopState manages the lifecycle of a goroutine running a loop in the background: starting it,
// nudging it to re-inspect the state it derives its behaviour from, and asking it to exit or
// waiting for it to exit on its own. The goroutine waits for its next signal with await. The zero
// value is ready to use.
//
// begin, interrupt and awaitEnd must be called by a single goroutine at a time; nudge is safe for
// concurrent use.
type loopState struct {
	mu sync.Mutex
	// active is true from begin until the goroutine calls end.
	active bool
	// stop is closed to ask the goroutine to exit; done is closed by end once it has.
	stop chan struct{}
	done chan struct{}
	// wake (buffered) nudges the goroutine to re-inspect state.
	wake chan struct{}
}

// begin marks the loop active, setting up fresh signalling channels for the new goroutine. The
// caller must have interrupted the previous goroutine first.
func (l *loopState) begin() {
	l.mu.Lock()
	defer l.mu.Unlock()
	// Assertion, not an error path: every caller holds the clock's driver mutex and interrupts
	// before beginning, so an active goroutine here means that invariant has been broken.
	if l.active {
		panic("cron: internal error: loopState.begin called while a goroutine is still active")
	}
	l.active = true
	l.stop = make(chan struct{})
	l.done = make(chan struct{})
	l.wake = make(chan struct{}, 1)
}

// end releases whoever is waiting for the goroutine to exit. The goroutine must call it exactly
// once, right before returning.
func (l *loopState) end() {
	l.mu.Lock()
	l.active = false
	done := l.done
	l.mu.Unlock()
	close(done)
}

// await waits for the next event the goroutine should react to: a nudge, the expiry of due (nil
// when there is no deadline) or an interrupt. It reports whether the goroutine should keep
// running: false means interrupt asked it to exit.
func (l *loopState) await(due <-chan time.Time) bool {
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
func (l *loopState) nudge() {
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
func (l *loopState) interrupt() {
	l.mu.Lock()
	if !l.active {
		l.mu.Unlock()
		return
	}
	close(l.stop)
	done := l.done
	l.mu.Unlock()
	<-done
}

// awaitEnd blocks until no goroutine is active, without asking it to exit.
func (l *loopState) awaitEnd() {
	l.mu.Lock()
	if !l.active {
		l.mu.Unlock()
		return
	}
	done := l.done
	l.mu.Unlock()
	<-done
}
