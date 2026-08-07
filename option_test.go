package cron

import (
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestWithClock(t *testing.T) {
	clock := NewDefaultClock(time.UTC, DefaultNopTimer)
	c := New(WithClock(clock))
	if c.clock != clock {
		t.Error("expected provided clock")
	}
}

func TestWithVerboseLogger(t *testing.T) {
	var buf syncWriter
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	c := New(WithLogger(logger))
	if c.logger != logger {
		t.Error("expected provided logger")
	}

	sched, err := standardParser.Parse("@every 1s")
	if err != nil {
		t.Error("non-nil error")
	}
	_, err = c.Schedule(sched, func() {})
	if err != nil {
		t.Error("non-nil error")
	}
	c.Start()
	time.Sleep(OneSecond)
	c.Stop()
	out := buf.String()
	if !strings.Contains(out, "start") ||
		!strings.Contains(out, "run") {
		t.Error("expected to see some actions, got:", out)
	}
}

func TestWithOnCycleCompleted(t *testing.T) {
	var buf syncWriter
	f := func() {
		_, err := buf.Write([]byte{'E', 'N', 'D'})
		if err != nil {
			t.Error("non-nil error")
		}
	}
	c := New(WithOnCycleCompleted(f))

	sched, err := standardParser.Parse("@every 1s")
	if err != nil {
		t.Error("non-nil error")
	}
	_, err = c.Schedule(sched, func() {
		_, err := buf.Write([]byte{'1'})
		if err != nil {
			t.Error("non-nil error")
		}
	})
	if err != nil {
		t.Error("non-nil error")
	}
	_, err = c.Schedule(sched, func() {
		_, err := buf.Write([]byte{'2'})
		if err != nil {
			t.Error("non-nil error")
		}
	})
	if err != nil {
		t.Error("non-nil error")
	}
	c.Start()
	time.Sleep(OneSecond)
	c.Stop()
	out := buf.String()
	if !strings.HasSuffix(out, "END") {
		t.Error("expected callback to be executed after jobs completed")
	}
}

// A job spanning the next activation gets that activation skipped when the
// cron is configured with WithSkipIfRunning, while other entries of the same
// cron run independently. The TimerSkippingRealExecutionClock fires the
// activations while the jobs execute in real time, reproducing the overlap
// through the scheduler.
func TestWithSkipIfRunning(t *testing.T) {
	start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := NewTimerSkippingRealExecutionClock(start)
	crn := New(WithClock(clock), WithSkipIfRunning())

	// Exactly two activations, one second apart: 00:00:01 and 00:00:02.
	sched, err := secondParser.Parse("1,2 0 0 1 1 *")
	if err != nil {
		t.Fatal(err)
	}

	var slowRuns, fastRuns atomic.Int32
	if _, err := crn.Schedule(sched, func() {
		slowRuns.Add(1)
		time.Sleep(1500 * time.Millisecond) // spans the second activation
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := crn.Schedule(sched, func() {
		fastRuns.Add(1)
	}); err != nil {
		t.Fatal(err)
	}

	crn.Start()
	defer crn.Stop()

	// The first activation starts both jobs; the second fires one flowing
	// second later, while the slow job is still running: its execution must be
	// skipped, while the fast entry, whose first run completed, runs again.
	clock.AdvanceBy(2 * time.Second)
	clock.WaitForIdle()

	if n := slowRuns.Load(); n != 1 {
		t.Errorf("expected 1 slow execution with the overlapping one skipped, got %d", n)
	}
	if n := fastRuns.Load(); n != 2 {
		t.Errorf("expected 2 fast executions unaffected by the slow entry, got %d", n)
	}
}

// A job spanning the next activation gets that activation queued (run after
// the first completes) when the cron is configured with WithQueueIfRunning.
func TestWithQueueIfRunning(t *testing.T) {
	start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := NewTimerSkippingRealExecutionClock(start)
	crn := New(WithClock(clock), WithQueueIfRunning())

	// Exactly two activations, one second apart: 00:00:01 and 00:00:02.
	sched, err := secondParser.Parse("1,2 0 0 1 1 *")
	if err != nil {
		t.Fatal(err)
	}

	const jobDuration = 1500 * time.Millisecond // spans the second activation
	var mu sync.Mutex
	var starts []time.Time
	if _, err := crn.Schedule(sched, func() {
		mu.Lock()
		starts = append(starts, clock.Now())
		mu.Unlock()
		time.Sleep(jobDuration)
	}); err != nil {
		t.Fatal(err)
	}

	crn.Start()
	defer crn.Stop()

	// The first activation starts the job; the second fires one flowing second
	// later, while the job is still running, and must wait for it to complete.
	clock.AdvanceBy(2 * time.Second)
	clock.WaitForIdle()

	mu.Lock()
	defer mu.Unlock()
	if len(starts) != 2 {
		t.Fatalf("expected 2 executions, got %d", len(starts))
	}
	// The second execution was queued: it can only start once the first has
	// completed, a full jobDuration after the first start — not at the one
	// second cadence of the schedule.
	if gap := starts[1].Sub(starts[0]); gap < jobDuration-100*time.Millisecond {
		t.Errorf("expected second execution queued until the first completed (~%v later), started %v later", jobDuration, gap)
	}
}
