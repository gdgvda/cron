package cron

import "time"

// TimerSkippingRealExecutionClock is a manual clock that skips the wait between scheduled
// activations (like TimerSkippingInstantExecutionClock) but, unlike it, lets jobs execute in real
// time, in their own goroutines, while the clock flows along with them.
//
// AdvanceTo and AdvanceBy move the virtual clock to the requested instant, firing every scheduled
// activation reached along the way. Idle stretches are skipped by jumping straight to the next
// activation, but whenever triggered jobs are still executing the clock flows 1:1 with the wall
// clock, so that the jobs observe time passing, and activations fire as the flowing clock reaches
// them. The advance returns once the clock reaches the requested instant, which takes the real
// execution time of the job windows crossed along the way; jobs still running at that point keep
// executing while the clock keeps flowing, and the clock freezes as soon as no job is running,
// until the next manual advance.
//
// Use WaitForIdle to block until all started jobs have completed and the clock is frozen.
//
// The clock is safe for concurrent use by the scheduler and the jobs it runs; AdvanceTo, AdvanceBy
// and WaitForIdle, however, are meant to be driven from a single goroutine.
type TimerSkippingRealExecutionClock struct {
	time  *tsrecVirtualTime
	timer *tsrecSchedulerTimer
	// cycles counts the fired activations whose jobs have not yet completed.
	cycles *tsrecCycleCounter
	// loop manages the goroutine that executes runLoop during and after each manual advance.
	loop *tsrecLoopState
}

func NewTimerSkippingRealExecutionClock(start time.Time) *TimerSkippingRealExecutionClock {
	return &TimerSkippingRealExecutionClock{
		time:   newTSRECVirtualTime(start),
		timer:  &tsrecSchedulerTimer{},
		cycles: newTSRECCycleCounter(),
		loop:   &tsrecLoopState{},
	}
}

func (c *TimerSkippingRealExecutionClock) Register(cron *Cron) []Option {
	return []Option{WithOnCycleCompleted(c.completeCycle)}
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
// the interval. It returns once the clock has reached the requested instant, which takes the real
// execution time of the job windows crossed along the way; jobs may still be running when it
// returns.
func (c *TimerSkippingRealExecutionClock) AdvanceBy(duration time.Duration) {
	c.loop.interrupt()
	c.advance(c.time.now().Add(duration))
}

// AdvanceTo moves the virtual clock forward to target, firing every activation scheduled up to and
// including it. It returns once the clock has reached target, which takes the real execution time
// of the job windows crossed along the way; jobs may still be running when it returns.
func (c *TimerSkippingRealExecutionClock) AdvanceTo(target time.Time) {
	c.loop.interrupt()
	c.advance(target)
}

// WaitForIdle blocks until every job started by fired activations has completed and the clock is
// frozen again. Call it after AdvanceTo/AdvanceBy to synchronize with the jobs they triggered.
// There is no timeout: it blocks forever if a job never returns, or if jobs outlast the interval
// to the next activation, so that the flowing clock keeps starting new ones and never freezes.
func (c *TimerSkippingRealExecutionClock) WaitForIdle() {
	c.cycles.awaitNone()
	c.loop.awaitEnd()
}

// advance starts the runLoop goroutine for an advance to target and returns once the clock has
// reached it: every activation scheduled up to target has fired. The goroutine may outlive the
// call, keeping the clock flowing until every started job has completed.
func (c *TimerSkippingRealExecutionClock) advance(target time.Time) {
	if !c.loop.begin() {
		return
	}
	reached := make(chan struct{})
	go c.runLoop(target, reached)
	<-reached
}

// runLoop moves the virtual clock to target and beyond: while no job is running the clock is
// frozen and jumps straight over idle time; while fired jobs are executing it flows 1:1 with the
// wall clock so that they observe time passing, and activations fire as the clock reaches them.
// reached is closed as soon as the clock reaches target, unblocking advance. runLoop returns —
// freezing the clock at the virtual time reached — once target is reached and no job is running,
// or when the next manual advance interrupts it.
func (c *TimerSkippingRealExecutionClock) runLoop(target time.Time, reached chan<- struct{}) {
	defer func() {
		c.time.freeze()
		c.loop.end()
	}()

	targetReached := false
	for {
		at, armed := c.timer.state()

		// The scheduler is not sleeping, processing its last wake-up: nothing can be concluded about the
		// remaining activations until it arms its next timer, which nudges the loop back here.
		if !armed {
			if !c.loop.await(nil) {
				return
			}
			continue
		}

		// The scheduler sleeps and no job is executing: the clock freezes and can jump over the
		// idle time.
		if c.cycles.running() == 0 {
			c.time.freeze()
			if at.IsZero() || at.After(target) {
				// No activation fits the advance: jump to target, completing the advance, and
				// exit — the deferred freeze pins the clock until the next one.
				c.time.advance(target)
				if !targetReached {
					close(reached)
				}
				return
			}
			// Jump straight to the armed activation and fire it; the jobs it starts, if any, set
			// the clock flowing from that very instant.
			c.time.advance(at)
			c.fire(at)
			if c.cycles.running() > 0 {
				c.time.flow()
			}
			continue
		}

		// The scheduler sleeps while jobs are executing: the clock flows with the wall clock so
		// that the jobs observe time passing.
		c.time.flow()
		now := c.time.now()

		// The advance is complete as soon as the flowing clock crosses target.
		if !targetReached && !now.Before(target) {
			targetReached = true
			close(reached)
		}

		// The armed activation fires as soon as the flowing clock reaches it.
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
// beforehand. The cycle whose jobs are about to start is counted as running before the scheduler
// is woken, so that it can never be observed completing before it started.
func (c *TimerSkippingRealExecutionClock) fire(at time.Time) {
	c.cycles.started()
	if !c.timer.fireIfArmedAt(at) {
		// The scheduler disarmed or re-armed its timer concurrently: no cycle started after all.
		c.cycles.completed()
	}
}

// completeCycle records that all jobs started by one activation have finished. The scheduler
// invokes it from a dedicated goroutine, so it never blocks the scheduler loop.
func (c *TimerSkippingRealExecutionClock) completeCycle() {
	c.cycles.completed()
	c.loop.nudge()
}
