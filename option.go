package cron

import (
	"log/slog"
)

// Option represents a modification to the default behavior of a Cron.
type Option func(*Cron)

// WithChain specifies Job wrappers to apply to all jobs added to this cron.
// Refer to the Chain* functions in this package for provided wrappers.
func WithChain(wrappers ...JobWrapper) Option {
	return func(c *Cron) {
		c.chain = NewChain(wrappers...)
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
