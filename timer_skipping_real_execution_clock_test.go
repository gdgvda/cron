package cron

import (
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
