package cron

import (
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestTimerSkippingRealExecutionClockStartTime(t *testing.T) {
	start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	c := NewTimerSkippingRealExecutionClock(start)
	if !c.Now().Equal(start) {
		t.Errorf("expected %v, got %v", start, c.Now())
	}
}

func TestTimerSkippingRealExecutionClockRegister(t *testing.T) {
	crn := New()
	c := NewTimerSkippingRealExecutionClock(time.Now())
	options := c.Register(crn)
	if len(options) != 1 {
		t.Fatalf("expected 1 option, got %d", len(options))
	}
	options[0](crn)
	if len(crn.onCycleCompleted) != 1 {
		t.Error("expected option to register an onCycleCompleted callback")
	}
}

// Advancing skips idle time and fires the scheduled activation; the job then runs in real time.
func TestTimerSkippingRealExecutionClockSkipsIdleTime(t *testing.T) {
	start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := NewTimerSkippingRealExecutionClock(start)
	crn := New(WithClock(clock))

	sched, err := secondParser.Parse("0 0 12 * * *") // daily at noon
	if err != nil {
		t.Fatal(err)
	}

	var ran bool
	if _, err = crn.Schedule(sched, func() { ran = true }); err != nil {
		t.Fatal(err)
	}

	crn.Start()
	defer crn.Stop()

	scheduled := time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC)
	realBefore := time.Now()
	clock.AdvanceTo(scheduled)
	clock.WaitForIdle()

	if !ran {
		t.Error("expected job to have run")
	}
	if elapsed := time.Since(realBefore); elapsed > 500*time.Millisecond {
		t.Errorf("expected timer to fire quickly, took %v", elapsed)
	}
	if clock.Now().Before(scheduled) {
		t.Errorf("expected clock at or after %v, got %v", scheduled, clock.Now())
	}
}

// Now() should advance in real time while a job is executing.
func TestTimerSkippingRealExecutionClockTracksRealTimeWhileExecuting(t *testing.T) {
	start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := NewTimerSkippingRealExecutionClock(start)
	crn := New(WithClock(clock))

	sched, err := secondParser.Parse("0 0 12 * * *") // daily at noon
	if err != nil {
		t.Fatal(err)
	}

	jobDuration := 60 * time.Millisecond
	var nowAtEnd time.Time
	if _, err = crn.Schedule(sched, func() {
		time.Sleep(jobDuration)
		nowAtEnd = clock.Now()
	}); err != nil {
		t.Fatal(err)
	}

	crn.Start()
	defer crn.Stop()

	// Advance to noon: the job starts immediately (timer-skipping). AdvanceTo returns without waiting
	// for it; WaitForIdle blocks until it completes, at which point nowAtEnd is set.
	scheduled := time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC)
	clock.AdvanceTo(scheduled)
	clock.WaitForIdle()

	// Now() at the end of the job should be noon + ~jobDuration.
	elapsed := nowAtEnd.Sub(scheduled)
	if elapsed < jobDuration/2 {
		t.Errorf("expected clock to advance ~%v during job execution, got %v", jobDuration, elapsed)
	}
	if elapsed > jobDuration*3 {
		t.Errorf("clock advanced too far during job execution: %v", elapsed)
	}
}

// When no jobs are scheduled (NopTimer path), Now() should be completely frozen.
func TestTimerSkippingRealExecutionClockFrozenWithNoJobs(t *testing.T) {
	start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := NewTimerSkippingRealExecutionClock(start)
	crn := New(WithClock(clock))
	// No jobs scheduled — NopTimer is used, no cycle ever starts.

	crn.Start()
	defer crn.Stop()

	time.Sleep(30 * time.Millisecond)

	if !clock.Now().Equal(start) {
		t.Errorf("expected clock frozen at %v with no jobs, got %v", start, clock.Now())
	}
}

// An advance spanning multiple activations fires them all, executing each job in real time.
func TestTimerSkippingRealExecutionClockMultipleCycles(t *testing.T) {
	start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := NewTimerSkippingRealExecutionClock(start)
	crn := New(WithClock(clock))

	sched, err := secondParser.Parse("* * * * * *") // every second
	if err != nil {
		t.Fatal(err)
	}

	const jobDuration = 80 * time.Millisecond
	var execCount atomic.Int32
	if _, err = crn.Schedule(sched, func() {
		execCount.Add(1)
		time.Sleep(jobDuration)
	}); err != nil {
		t.Fatal(err)
	}

	crn.Start()
	defer crn.Stop()

	// Advance 3 seconds: fires activations at +1s, +2s, +3s. Each job runs for 80ms of real time
	// with the clock flowing, then the clock jumps to the next activation. Wait until all complete
	// before asserting.
	clock.AdvanceBy(3 * time.Second)
	clock.WaitForIdle()

	if n := execCount.Load(); n < 2 {
		t.Errorf("expected at least 2 executions, got %d", n)
	}
}

// A job outlasting the interval to the next activation keeps the clock flowing into it: the next
// activation fires while the job is still running (overlapping cycles). WaitForIdle is guarded by
// a timeout since it would block forever if jobs never stopped outlasting the interval.
func TestTimerSkippingRealExecutionClockOverlappingCycles(t *testing.T) {
	start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := NewTimerSkippingRealExecutionClock(start)
	crn := New(WithClock(clock))

	sched, err := secondParser.Parse("* * * * * *") // every second
	if err != nil {
		t.Fatal(err)
	}

	// While flowing, activations are one real second apart: the first three jobs outlast the
	// interval, chaining the overlap across cycles; the following ones return at once so the
	// clock can freeze.
	var execs atomic.Int32
	if _, err = crn.Schedule(sched, func() {
		if execs.Add(1) <= 3 {
			time.Sleep(1500 * time.Millisecond)
		}
	}); err != nil {
		t.Fatal(err)
	}

	crn.Start()
	defer crn.Stop()

	clock.AdvanceBy(time.Second)

	done := make(chan struct{})
	go func() {
		clock.WaitForIdle()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("expected WaitForIdle to return once jobs stop outlasting the activation interval")
	}

	// activation:  00:01    00:02     00:03     00:04        (00:05 never reached)
	// job 1          ████████████████                        (00:01 → 00:02.5)       execs=1
	// job 2                   ████████████████               (00:02 → 00:03.5)       execs=2
	// job 3                             ████████████████     (00:03 → 00:04.5)       execs=3
	// job 4                                       ▏          (00:04, instant)        execs=4

	// The clock flows only while jobs run, so each execution after the first proves an activation
	// fired while an earlier job was still running: +2s during job 1, +3s during job 2 (and 1),
	// +4s during job 3 (and 2).
	if n := execs.Load(); n < 4 {
		t.Errorf("expected activations to keep firing while earlier jobs were running, got %d executions", n)
	}
}

// A job triggered in the middle of an advance observes flowing time while it executes: the clock
// must not jump to the following activations until the job has completed.
func TestTimerSkippingRealExecutionClockFlowsDuringJumpWindows(t *testing.T) {
	start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := NewTimerSkippingRealExecutionClock(start)
	crn := New(WithClock(clock))

	sched, err := secondParser.Parse("* * * * * *") // every second
	if err != nil {
		t.Fatal(err)
	}

	const jobDuration = 50 * time.Millisecond
	var mu sync.Mutex
	var ends []time.Time
	if _, err = crn.Schedule(sched, func() {
		time.Sleep(jobDuration)
		mu.Lock()
		ends = append(ends, clock.Now())
		mu.Unlock()
	}); err != nil {
		t.Fatal(err)
	}

	crn.Start()
	defer crn.Stop()

	clock.AdvanceBy(3 * time.Second)
	clock.WaitForIdle()

	mu.Lock()
	defer mu.Unlock()
	if len(ends) == 0 {
		t.Fatal("expected at least one execution")
	}
	// The first job fired at +1s and ran for jobDuration with the clock flowing: at its end the
	// clock must sit shortly after its activation, not at a later activation or at target.
	elapsed := ends[0].Sub(start.Add(time.Second))
	if elapsed < jobDuration/2 {
		t.Errorf("expected the clock to flow ~%v during the job, got %v", jobDuration, elapsed)
	}
	if elapsed >= time.Second {
		t.Errorf("expected the clock not to jump past the next activation during the job, got +1s+%v", elapsed)
	}
}

// Concurrent advances are serialized rather than silently dropped: each one runs to completion, so
// their durations add up and every activation in between fires.
func TestTimerSkippingRealExecutionClockConcurrentAdvances(t *testing.T) {
	start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := NewTimerSkippingRealExecutionClock(start)
	crn := New(WithClock(clock))

	sched, err := secondParser.Parse("* * * * * *") // every second
	if err != nil {
		t.Fatal(err)
	}

	const advances = 4
	var execs atomic.Int32
	if _, err = crn.Schedule(sched, func() { execs.Add(1) }); err != nil {
		t.Fatal(err)
	}

	crn.Start()
	defer crn.Stop()

	var wg sync.WaitGroup
	for i := 0; i < advances; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			clock.AdvanceBy(time.Second)
		}()
	}
	wg.Wait()
	clock.WaitForIdle()

	if target := start.Add(advances * time.Second); clock.Now().Before(target) {
		t.Errorf("expected concurrent advances to add up to %v, got %v", target, clock.Now())
	}
	if n := execs.Load(); n < advances {
		t.Errorf("expected %d activations to fire, got %d executions", advances, n)
	}
}

// Firing an activation the scheduler has already stopped waiting for must not leave a cycle
// counted as running, which would hang WaitForIdle forever.
func TestTimerSkippingRealExecutionClockFireCompensatesLostRace(t *testing.T) {
	start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := NewTimerSkippingRealExecutionClock(start)

	at := start.Add(time.Second)
	clock.timer.arm(at)
	clock.timer.disarm() // the scheduler woke up for another reason, e.g. an entry insertion

	clock.fire(at)

	if n := clock.cycles.running(); n != 0 {
		t.Errorf("expected no cycle running after a lost fire race, got %d", n)
	}
}

// WaitForIdle on a clock that never fired anything returns immediately.
func TestTimerSkippingRealExecutionClockWaitForIdleWhenAlreadyIdle(t *testing.T) {
	clock := NewTimerSkippingRealExecutionClock(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
	done := make(chan struct{})
	go func() {
		clock.WaitForIdle()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("WaitForIdle should return immediately on an idle clock")
	}
}

func TestVirtualTimeFrozenByDefault(t *testing.T) {
	start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	v := newVirtualTime(start)

	time.Sleep(20 * time.Millisecond)

	if !v.now().Equal(start) {
		t.Errorf("expected time frozen at %v, got %v", start, v.now())
	}
}

func TestVirtualTimeAdvanceMovesForwardOnly(t *testing.T) {
	start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	v := newVirtualTime(start)

	future := start.Add(time.Hour)
	v.advance(future)
	if !v.now().Equal(future) {
		t.Errorf("expected time advanced to %v, got %v", future, v.now())
	}

	v.advance(start)
	if !v.now().Equal(future) {
		t.Errorf("expected advance never to move time backwards, got %v", v.now())
	}
}

func TestVirtualTimeFlowTracksWallClock(t *testing.T) {
	start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	v := newVirtualTime(start)

	v.flow()
	const sleep = 50 * time.Millisecond
	time.Sleep(sleep)

	elapsed := v.now().Sub(start)
	if elapsed < sleep/2 {
		t.Errorf("expected flowing time to track the wall clock, only %v elapsed", elapsed)
	}
	if elapsed > 10*sleep {
		t.Errorf("flowing time ran too fast: %v elapsed", elapsed)
	}
}

func TestVirtualTimeAdvanceIgnoredWhileFlowing(t *testing.T) {
	start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	v := newVirtualTime(start)

	v.flow()
	v.advance(start.Add(time.Hour))

	if v.now().Sub(start) >= time.Hour {
		t.Errorf("expected advance to be a no-op while flowing, got %v", v.now())
	}
}

func TestVirtualTimeFreezePinsCurrentInstant(t *testing.T) {
	start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	v := newVirtualTime(start)

	v.flow()
	time.Sleep(20 * time.Millisecond)
	v.freeze()

	pinned := v.now()
	if pinned.Before(start) {
		t.Errorf("expected frozen time at or after %v, got %v", start, pinned)
	}
	time.Sleep(20 * time.Millisecond)
	if !v.now().Equal(pinned) {
		t.Errorf("expected time to stay pinned at %v after freeze, got %v", pinned, v.now())
	}
}

func TestSchedulerTimerArmAndFire(t *testing.T) {
	s := &schedulerTimer{}
	at := time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC)

	timer := s.arm(at)
	if !s.fireIfArmedAt(at) {
		t.Fatal("expected fire to succeed on a timer armed at the same time")
	}

	select {
	case <-timer:
	default:
		t.Error("expected the timer channel to have fired")
	}
	if _, open := <-timer; open {
		t.Error("expected the timer channel to be closed after firing")
	}
}

func TestSchedulerTimerFireRequiresMatchingActivation(t *testing.T) {
	s := &schedulerTimer{}
	at := time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC)

	s.arm(at)
	if s.fireIfArmedAt(at.Add(time.Second)) {
		t.Error("expected fire to fail for a different activation time")
	}
	if !s.fireIfArmedAt(at) {
		t.Error("expected fire to succeed for the armed activation time")
	}
	if s.fireIfArmedAt(at) {
		t.Error("expected fire to fail once the timer has already fired")
	}
}

func TestSchedulerTimerFireFailsWhenDisarmed(t *testing.T) {
	s := &schedulerTimer{}
	at := time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC)

	s.arm(at)
	s.disarm()
	if s.fireIfArmedAt(at) {
		t.Error("expected fire to fail on a disarmed timer")
	}
}

func TestSchedulerTimerState(t *testing.T) {
	s := &schedulerTimer{}
	at := time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC)

	if _, armed := s.state(); armed {
		t.Error("expected the timer not to be armed initially")
	}

	s.arm(at)
	if got, armed := s.state(); !armed || !got.Equal(at) {
		t.Errorf("expected an armed timer at %v, got %v (armed=%v)", at, got, armed)
	}

	s.disarm()
	if _, armed := s.state(); armed {
		t.Error("expected the timer not to be armed after disarming")
	}
}

func TestSchedulerTimerNop(t *testing.T) {
	s := &schedulerTimer{}

	timer := s.armNop()
	if at, armed := s.state(); !armed || !at.IsZero() {
		t.Errorf("expected an armed nop timer with a zero activation, got %v (armed=%v)", at, armed)
	}
	if s.fireIfArmedAt(time.Time{}) {
		t.Error("expected a nop timer never to fire")
	}
	select {
	case <-timer:
		t.Error("expected the nop timer channel never to fire")
	default:
	}
}

// Exercise the full arm/fire protocol between a scheduler-like goroutine and a firing goroutine,
// as the clock uses it.
func TestSchedulerTimerConcurrentArmAndFire(t *testing.T) {
	s := &schedulerTimer{}
	base := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	const rounds = 100

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < rounds; i++ {
			timer := s.arm(base.Add(time.Duration(i) * time.Second))
			<-timer
		}
	}()

	deadline := time.Now().Add(5 * time.Second)
	for fired := 0; fired < rounds; {
		if time.Now().After(deadline) {
			t.Fatalf("expected %d fires within the deadline, got %d", rounds, fired)
		}
		at, armed := s.state()
		if !armed {
			runtime.Gosched()
			continue
		}
		if !s.fireIfArmedAt(at) {
			t.Fatalf("fire %d: expected fire to succeed, the scheduler only re-arms after each fire", fired)
		}
		fired++
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("expected the scheduler goroutine to observe all fires")
	}
}

func TestCycleCounterCounts(t *testing.T) {
	c := newCycleCounter()
	if c.running() != 0 {
		t.Errorf("expected 0 running cycles, got %d", c.running())
	}

	c.started()
	c.started()
	if c.running() != 2 {
		t.Errorf("expected 2 running cycles, got %d", c.running())
	}

	c.completed()
	if c.running() != 1 {
		t.Errorf("expected 1 running cycle, got %d", c.running())
	}
}

func TestCycleCounterAwaitNoneReturnsImmediatelyAtZero(t *testing.T) {
	c := newCycleCounter()
	done := make(chan struct{})
	go func() {
		c.awaitNone()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("expected awaitNone to return immediately when no cycle is running")
	}
}

func TestCycleCounterAwaitNoneBlocksWhileRunning(t *testing.T) {
	c := newCycleCounter()
	c.started()

	done := make(chan struct{})
	go func() {
		c.awaitNone()
		close(done)
	}()

	select {
	case <-done:
		t.Fatal("expected awaitNone to block while a cycle is running")
	case <-time.After(30 * time.Millisecond):
	}

	c.completed()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("expected awaitNone to return once no cycle is running")
	}
}

func TestLoopStateBeginAgainAfterEnd(t *testing.T) {
	state := &loopState{}
	state.begin()
	state.end()
	state.begin()
	state.end()
}

// begin asserts that the previous goroutine was interrupted. The clock's driver mutex makes this
// unreachable through the public API; the assertion exists so that a future caller breaking the
// invariant fails loudly instead of leaking a goroutine.
func TestLoopStateBeginAssertsPreviousGoroutineEnded(t *testing.T) {
	state := &loopState{}
	state.begin()
	defer func() {
		if recover() == nil {
			t.Error("expected begin to panic while a goroutine is active")
		}
		state.end()
	}()
	state.begin()
}

func TestLoopStateInterruptStopsGoroutine(t *testing.T) {
	state := &loopState{}
	state.begin()

	exited := make(chan struct{})
	go func() {
		for state.await(nil) {
		}
		// exited is closed before end so that interrupt returning proves the goroutine exited.
		close(exited)
		state.end()
	}()

	state.interrupt()
	select {
	case <-exited:
	default:
		t.Error("expected interrupt to return only after the goroutine ended")
	}
}

func TestLoopStateInterruptAndAwaitEndWhenInactive(t *testing.T) {
	state := &loopState{}
	done := make(chan struct{})
	go func() {
		state.interrupt()
		state.awaitEnd()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("expected interrupt and awaitEnd to return immediately on an inactive loop")
	}
}

func TestLoopStateAwaitEnd(t *testing.T) {
	state := &loopState{}
	state.begin()

	finish := make(chan struct{})
	go func() {
		<-finish
		state.end()
	}()

	done := make(chan struct{})
	go func() {
		state.awaitEnd()
		close(done)
	}()

	select {
	case <-done:
		t.Fatal("expected awaitEnd to block while the goroutine is active")
	case <-time.After(30 * time.Millisecond):
	}

	close(finish)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("expected awaitEnd to return once the goroutine ended")
	}
}

func TestLoopStateAwaitDue(t *testing.T) {
	state := &loopState{}
	state.begin()

	wallTimer := time.NewTimer(time.Millisecond)
	defer wallTimer.Stop()
	if !state.await(wallTimer.C) {
		t.Error("expected await to keep running on wall-clock timer expiry")
	}
	state.end()
}

func TestLoopStateNudge(t *testing.T) {
	state := &loopState{}
	state.nudge() // inactive: dropped, must not panic

	state.begin()
	state.nudge()
	if !state.await(nil) {
		t.Fatal("expected await to keep running after a nudge")
	}
	state.end()
}
