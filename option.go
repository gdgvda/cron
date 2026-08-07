package cron

import (
	"log/slog"
	"sync"
	"time"
)

// Option represents a modification to the default behavior of a Cron.
type Option func(*Cron)

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

// WithSkipIfRunning skips an activation that comes due while the previous run of
// the same job is still in progress.
//
// The guard is per job, so distinct jobs never hold each other up. It is
// mutually exclusive with [WithQueueIfRunning]: giving both leaves the last
// one in effect. With neither, successive runs of the same job may overlap.
func WithSkipIfRunning() Option {
	return func(c *Cron) {
		c.overlap = func(cmd func(), logger *slog.Logger) func() {
			var ch = make(chan struct{}, 1)
			ch <- struct{}{}
			return func() {
				select {
				case v := <-ch:
					defer func() { ch <- v }()
					cmd()
				default:
					logger.Info("job execution skipped", "event", "skip")
				}
			}
		}
	}
}

// WithQueueIfRunning defers an activation that comes due while the previous
// run of the same job is still in progress, running it once that one completes.
//
// The guard is per job, so distinct jobs never hold each other up. It is
// mutually exclusive with [WithSkipIfRunning]: giving both leaves the last one
// in effect. With neither, successive runs of the same job may overlap.
func WithQueueIfRunning() Option {
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
