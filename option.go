package cron

import (
	"log/slog"
	"sync"
	"time"
)

// Option represents a modification to the default behavior of a Cron.
type Option func(*Cron)

// WithSeconds overrides the parser used for interpreting job schedules to
// include a seconds field as the first one.
func WithSeconds() Option {
	return WithParser(NewParser(
		Second | Minute | Hour | Dom | Month | Dow | Descriptor,
	))
}

// WithParser overrides the parser used for interpreting job schedules.
func WithParser(p Parser) Option {
	return func(c *Cron) {
		c.parser = p
	}
}

// WithLogger uses the provided logger.
func WithLogger(logger *slog.Logger) Option {
	return func(c *Cron) {
		c.logger = logger
	}
}

func WithClock(clock Clock) Option {
	return func(c *Cron) {
		options := clock.Register(c)
		for _, option := range options {
			option(c)
		}
		c.clock = clock
	}
}

// WithOnCycleCompleted registers a callback that will be executed every time all jobs executions that
// have been started in the same instant have completed.
func WithOnCycleCompleted(f func()) Option {
	return func(c *Cron) {
		c.onCycleCompleted = append(c.onCycleCompleted, f)
	}
}

func WithSkipIfRunning() Option {
	return func(c *Cron) {
		c.overlap = func(cmd func(), logger *slog.Logger) func() {
			var ch = make(chan struct{}, 1)
			ch <- struct{}{}
			return func() {
				select {
				case v := <-ch:
					defer func() { ch <- v }()
				default:
					logger.Info("job execution skipped", "event", "skip")
				}
			}
		}
	}
}

func WithQueueIfStillRunning() Option {
	return func(c *Cron) {
		c.overlap = func(cmd func(), logger *slog.Logger) func() {
			var mu sync.Mutex
			return func() {
				start := time.Now()
				mu.Lock()
				defer mu.Unlock()
				if dur := time.Since(start); dur > time.Minute {
					logger.Info("job execution delayed", "event", "delay", "duration", dur)
				}
				cmd()
			}
		}
	}
}
