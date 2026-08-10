package cron

import (
	"sync"
	"time"
)

const DefaultNopTimer = 100_000 * time.Hour

func NewDefaultClock(location *time.Location, nop time.Duration) *DefaultClock {
	return &DefaultClock{
		location: location,
		nop:      nop,
	}
}

type DefaultClock struct {
	location *time.Location
	nop      time.Duration
}

func (c *DefaultClock) Register(cron *Cron) []Option {
	return []Option{}
}

func (c *DefaultClock) Now() time.Time {
	return time.Now().In(c.location)
}

func (c *DefaultClock) Timer(t time.Time) (<-chan struct{}, func()) {
	return c.timer(time.Until(t))
}

func (c *DefaultClock) NopTimer() (<-chan struct{}, func()) {
	return c.timer(c.nop)
}

func (c *DefaultClock) timer(duration time.Duration) (<-chan struct{}, func()) {
	timer := time.NewTimer(duration)
	out := make(chan struct{}, 1)
	stop := make(chan struct{})
	go func() {
		defer timer.Stop()
		select {
		case <-timer.C:
			out <- struct{}{}
		case <-stop:
		}
	}()
	return out, sync.OnceFunc(func() { close(stop) })
}
