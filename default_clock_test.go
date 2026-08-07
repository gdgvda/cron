package cron

import (
	"testing"
	"time"
)

// The scheduler calls stop() whenever it wakes for a reason other than the timer. That can happen
// after the timer has already fired, with nobody left to receive the activation: stop() must still
// return, or the scheduler goroutine deadlocks.
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

			done := make(chan struct{})
			go func() {
				stop()
				close(done)
			}()

			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatal("stop() blocked forever")
			}
		})
	}
}
