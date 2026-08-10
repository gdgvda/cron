package cron

import (
	"testing"
	"time"
)

func TestStartTime(t *testing.T) {
	start := time.Now()
	c := NewTimerSkippingInstantExecutionClock(start)
	now := c.Now()
	if !now.Equal(start) {
		t.Errorf("expected: %v, got: %v", start, now)
	}
}

func TestRegister(t *testing.T) {
	crn := New()
	start := time.Now()
	c := NewTimerSkippingInstantExecutionClock(start)
	options := c.Register(crn)
	if options == nil {
		t.Error("expected non-nil options, got nil")
	}
	if len(options) != 1 {
		t.Errorf("expected 1 option, got %v", len(options))
	}
	options[0](crn)
	if len(crn.onCycleCompleted) != 1 {
		t.Error("option was expected to be an onCycleCompleted")
	}
}

func TestJobTriggeredAfterAdvancingOnce(t *testing.T) {
	start := time.Date(2025, time.October, 7, 12, 0, 0, 0, time.UTC)
	clock := NewTimerSkippingInstantExecutionClock(start)
	cron := New(WithClock(clock))
	executed := false
	sched, err := secondParser.Parse("1 0 12 * * *")
	if err != nil {
		t.Error("non-nil error")
	}
	_, err = cron.Schedule(sched, func() {
		executed = true
	})
	if err != nil {
		t.Error("non-nil error")
	}
	cron.Start()
	defer cron.Stop()
	time.Sleep(2 * time.Second)
	if executed {
		t.Error("expected timer not to trigger before advancing")
	}
	clock.AdvanceBy(2 * time.Second)
	if !executed {
		t.Error("expected timer to be triggered after advancing")
	}
}

func TestJobTriggeredAfterAdvancingTwice(t *testing.T) {
	start := time.Date(2025, time.October, 7, 12, 0, 0, 0, time.UTC)
	clock := NewTimerSkippingInstantExecutionClock(start)
	cron := New(WithClock(clock))
	executed := false
	sched, err := secondParser.Parse("2 0 12 * * *")
	if err != nil {
		t.Error("non-nil error")
	}
	_, err = cron.Schedule(sched, func() {
		executed = true
	})
	if err != nil {
		t.Error("non-nil error")
	}
	cron.Start()
	defer cron.Stop()
	time.Sleep(3 * time.Second)
	if executed {
		t.Error("expected timer not to trigger before advancing")
	}
	clock.AdvanceBy(time.Second)
	if executed {
		t.Error("expected timer not to trigger after advancing only once")
	}
	clock.AdvanceBy(time.Second)
	if !executed {
		t.Error("expected timer to be triggered after advancing twice")
	}
}

func TestAdvanceSurvivesAbandonedActivation(t *testing.T) {
	start := time.Date(2025, time.October, 7, 12, 0, 0, 0, time.UTC)
	clock := NewTimerSkippingInstantExecutionClock(start)
	clock.Register(New())

	at := start.Add(time.Second)
	timer, stop := clock.Timer(at)

	advanced := make(chan struct{})
	go func() {
		clock.AdvanceBy(time.Second)
		close(advanced)
	}()

	// Stand in for the scheduler waking on an insertion rather than on the timer: abandon the
	// activation it already fired, re-arm at the same still-due time, then take the re-fire.
	go func() {
		for len(timer) == 0 {
			time.Sleep(time.Millisecond)
		}
		stop()

		refired, _ := clock.Timer(at)
		<-refired
		clock.onCycleCompleted <- struct{}{}
		clock.Timer(at.Add(time.Second))
	}()

	select {
	case <-advanced:
	case <-time.After(2 * time.Second):
		t.Fatal("AdvanceBy blocked forever on a cycle completion the abandoned activation never produces")
	}
}

func TestNoJobsRegistered(t *testing.T) {
	start := time.Date(2025, time.October, 7, 12, 0, 0, 0, time.UTC)
	clock := NewTimerSkippingInstantExecutionClock(start)
	cron := New(WithClock(clock))
	cron.Start()
	defer cron.Stop()
	now := clock.Now()
	if !now.Equal(start) {
		t.Errorf("expected now: %v, got: %v", start, now)
	}
	next := time.Date(2025, time.October, 7, 18, 0, 0, 0, time.UTC)
	clock.AdvanceTo(next)
	now = clock.Now()
	if !now.Equal(next) {
		t.Errorf("expected now: %v, got: %v", start, next)
	}
}

func TestNoJobsInBetween(t *testing.T) {
	start := time.Date(2025, time.October, 7, 12, 0, 0, 0, time.UTC)
	clock := NewTimerSkippingInstantExecutionClock(start)
	cron := New(WithClock(clock))
	sched, err := secondParser.Parse("0 0 20 * * *")
	if err != nil {
		t.Error("non-nil error")
	}
	_, err = cron.Schedule(sched, func() {})
	if err != nil {
		t.Error("non-nil error")
	}
	cron.Start()
	defer cron.Stop()
	now := clock.Now()
	if !now.Equal(start) {
		t.Errorf("expected now: %v, got: %v", start, now)
	}
	next := time.Date(2025, time.October, 7, 18, 0, 0, 0, time.UTC)
	clock.AdvanceTo(next)
	now = clock.Now()
	if !now.Equal(next) {
		t.Errorf("expected now: %v, got: %v", start, next)
	}
}
