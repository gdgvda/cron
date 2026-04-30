package cron

import (
	"sync"
	"time"
)

type TimerSkippingInstantExecutionClock struct {
	mu   *sync.Mutex
	now  time.Time
	next struct {
		time  time.Time
		timer chan struct{}
	}
	sleeping struct {
		value     bool
		condition *sync.Cond
	}
	onCycleCompleted chan struct{}
}

func NewTimerSkippingInstantExecutionClock(start time.Time) *TimerSkippingInstantExecutionClock {
	l := &sync.Mutex{}
	return &TimerSkippingInstantExecutionClock{
		mu:  l,
		now: start,
		next: struct {
			time  time.Time
			timer chan struct{}
		}{
			time:  time.Time{},
			timer: nil,
		},
		sleeping: struct {
			value     bool
			condition *sync.Cond
		}{
			value:     false,
			condition: sync.NewCond(l),
		},
	}
}

func (c *TimerSkippingInstantExecutionClock) Register(cron *Cron) []Option {
	c.onCycleCompleted = make(chan struct{})
	return []Option{WithOnCycleCompleted(
		func() {
			c.onCycleCompleted <- struct{}{}
		},
	)}
}

func (c *TimerSkippingInstantExecutionClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *TimerSkippingInstantExecutionClock) Timer(t time.Time) (<-chan struct{}, func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.next.time = t
	c.next.timer = make(chan struct{}, 1)
	c.sleeping.value = true
	c.sleeping.condition.Signal()
	return c.next.timer, func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		c.sleeping.value = false
	}
}

func (c *TimerSkippingInstantExecutionClock) NopTimer() (<-chan struct{}, func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.next.time = time.Time{}
	c.sleeping.value = true
	c.sleeping.condition.Signal()
	out := make(chan struct{})
	return out, func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		c.sleeping.value = false
	}
}

func (c *TimerSkippingInstantExecutionClock) AdvanceBy(duration time.Duration) {
	target := c.now.Add(duration)
	c.AdvanceTo(target)
}

func (c *TimerSkippingInstantExecutionClock) AdvanceTo(target time.Time) {
	for {
		c.mu.Lock()
		for !c.sleeping.value {
			c.sleeping.condition.Wait()
		}
		if c.next.time.IsZero() || c.next.time.After(target) {
			c.now = target
			c.mu.Unlock()
			return
		}
		c.now = c.next.time
		c.sleeping.value = false
		c.next.timer <- struct{}{}
		close(c.next.timer)
		c.mu.Unlock()
		<-c.onCycleCompleted
	}
}
