package cron

import "time"

type Clock interface {
	Now() time.Time
	Timer(time.Time) (timer <-chan struct{}, stop func())
}

type DefaultClock struct {
	location *time.Location
}

func NewDefaultClock(location *time.Location) *DefaultClock {
	return &DefaultClock{
		location: location,
	}
}

func (c *DefaultClock) Now() time.Time {
	return time.Now().In(c.location)
}

func (c *DefaultClock) Timer(t time.Time) (<-chan struct{}, func()) {
	timer := time.NewTimer(time.Until(t))
	out := make(chan struct{})
	stop := make(chan struct{})
	go func() {
		select {
		case <-timer.C:
			out <- struct{}{}
		case <-stop:
		}
	}()
	return out, func() {
		stop <- struct{}{}
	}
}
