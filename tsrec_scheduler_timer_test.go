package cron

import (
	"runtime"
	"testing"
	"time"
)

func TestTSRECSchedulerTimerArmAndFire(t *testing.T) {
	s := &tsrecSchedulerTimer{}
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

func TestTSRECSchedulerTimerFireRequiresMatchingActivation(t *testing.T) {
	s := &tsrecSchedulerTimer{}
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

func TestTSRECSchedulerTimerFireFailsWhenDisarmed(t *testing.T) {
	s := &tsrecSchedulerTimer{}
	at := time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC)

	s.arm(at)
	s.disarm()
	if s.fireIfArmedAt(at) {
		t.Error("expected fire to fail on a disarmed timer")
	}
}

func TestTSRECSchedulerTimerState(t *testing.T) {
	s := &tsrecSchedulerTimer{}
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

func TestTSRECSchedulerTimerNop(t *testing.T) {
	s := &tsrecSchedulerTimer{}

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
func TestTSRECSchedulerTimerConcurrentArmAndFire(t *testing.T) {
	s := &tsrecSchedulerTimer{}
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
