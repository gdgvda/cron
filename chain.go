package cron

import (
	"fmt"
	"log/slog"
	"runtime"
	"sync"
	"time"
)

// JobWrapper decorates the given job with some behavior.
type JobWrapper func(func()) func()

// Chain is a sequence of JobWrappers that decorates submitted jobs with
// cross-cutting behaviors like logging or synchronization.
type Chain struct {
	wrappers []JobWrapper
}

// NewChain returns a Chain consisting of the given JobWrappers.
func NewChain(c ...JobWrapper) Chain {
	return Chain{c}
}

// Then decorates the given job with all JobWrappers in the chain.
//
// This:
//
//	NewChain(m1, m2, m3).Then(job)
//
// is equivalent to:
//
//	m1(m2(m3(job)))
func (c Chain) Then(job func()) func() {
	for i := range c.wrappers {
		job = c.wrappers[len(c.wrappers)-i-1](job)
	}
	return job
}

// Recover panics in wrapped jobs and log them with the provided logger.
func Recover(logger *slog.Logger) JobWrapper {
	return func(job func()) func() {
		return func() {
			defer func() {
				if r := recover(); r != nil {
					const size = 64 << 10
					buf := make([]byte, size)
					buf = buf[:runtime.Stack(buf, false)]
					err, ok := r.(error)
					if !ok {
						err = fmt.Errorf("%v", r)
					}
					logger.Error(err.Error(), "event", "panic", "stack", "...\n"+string(buf))
				}
			}()
			job()
		}
	}
}

// DelayIfStillRunning serializes jobs, delaying subsequent runs until the
// previous one is complete. Jobs running after a delay of more than a minute
// have the delay logged at Info.
func DelayIfStillRunning(logger *slog.Logger) JobWrapper {
	return func(job func()) func() {
		var mu sync.Mutex
		return func() {
			start := time.Now()
			mu.Lock()
			defer mu.Unlock()
			if dur := time.Since(start); dur > time.Minute {
				logger.Info("job execution delayed", "event", "delay", "duration", dur)
			}
			job()
		}
	}
}

// SkipIfStillRunning skips an invocation of the job if a previous invocation is
// still running. It logs skips to the given logger at Info level.
func SkipIfStillRunning(logger *slog.Logger) JobWrapper {
	return func(job func()) func() {
		var ch = make(chan struct{}, 1)
		ch <- struct{}{}
		return func() {
			select {
			case v := <-ch:
				defer func() { ch <- v }()
				job()
			default:
				logger.Info("job execution skipped", "event", "skip")
			}
		}
	}
}
