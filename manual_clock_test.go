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
