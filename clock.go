package cron

import "time"

type Timer interface {
	Stop() bool
	Reset(d time.Duration) bool
	Chan() <-chan time.Time
}

type FireableTimer interface {
	Timer
	Fire()
}

type Clock interface {
	Now() time.Time
	Timer(time.Time) Timer
	SetTime(time.Time) bool
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

func (c *DefaultClock) Timer(t time.Time) Timer {
	return NewStdTimer(time.NewTimer(t.Sub(c.Now())))
}

func (c *DefaultClock) SetTime(t time.Time) bool {
	return false
}

type StdTimer struct {
	*time.Timer
}

func NewStdTimer(t *time.Timer) *StdTimer {
	return &StdTimer{
		Timer: t,
	}
}

func (t *StdTimer) Chan() <-chan time.Time {
	return t.C
}

type FakeTimer struct {
	clock  *FakeClock
	expire time.Time
	C      chan time.Time
	fired  bool
}

func (t *FakeTimer) Stop() bool {
	if t.fired {
		return false
	}
	t.fired = true
	return true
}

func (t *FakeTimer) Chan() <-chan time.Time {
	return t.C
}

func (t *FakeTimer) Reset(d time.Duration) bool {
	if t.fired {
		return false
	}
	t.expire = t.clock.Now().Add(d)
	return true
}

func (t *FakeTimer) Fire() {
	if t.fired {
		panic("timer already fired")
	}
	t.fired = true
	t.C <- t.expire
}

type FakeClock struct {
	location *time.Location
	current  time.Time
}

func NewFakeClock(location *time.Location, current time.Time) *FakeClock {
	return &FakeClock{
		location: location,
		current:  current.In(location),
	}
}

func (c *FakeClock) Now() time.Time {
	return c.current.In(c.location)
}

func (c *FakeClock) Timer(t time.Time) Timer {
	return &FakeTimer{
		clock:  c,
		expire: t,
		C:      make(chan time.Time),
	}
}

func (c *FakeClock) SetTime(t time.Time) bool {
	c.current = t
	return true
}
