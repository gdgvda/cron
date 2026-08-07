package cron

import (
	"testing"
	"time"
)

// stopCompletesWithin isolates stop in its own goroutine, so a deadlock fails the test instead of the package.
func stopCompletesWithin(t *testing.T, stop func(), timeout time.Duration) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(timeout):
		t.Fatal("stop() blocked forever")
	}
}

func TestDefaultClockStopDoesNotBlock(t *testing.T) {
	cases := []struct {
		name   string
		settle time.Duration
	}{
		{"before the timer fires", 0},
		{"after the timer fired unobserved", 50 * time.Millisecond},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			clock := NewDefaultClock(time.UTC, DefaultNopTimer)
			_, stop := clock.Timer(clock.Now().Add(5 * time.Millisecond))

			time.Sleep(c.settle)

			stopCompletesWithin(t, stop, 2*time.Second)
		})
	}
}

func TestDefaultClockStopIsIdempotent(t *testing.T) {
	clock := NewDefaultClock(time.UTC, DefaultNopTimer)
	_, stop := clock.Timer(clock.Now().Add(time.Hour))

	stopCompletesWithin(t, stop, 2*time.Second)
	stopCompletesWithin(t, stop, 2*time.Second)
}
