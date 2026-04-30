package cron

import "time"

// The cron scheduler implements the following cycle:
//   - next job time evaluation
//   - timer setup: clock.NopTimer() when there are no jobs registered, clock.Timer() with the
//     next activation time otherwise
//   - wakes up when timer fires or other actions are triggered (e.g. insertion, removal): handles event and move
//     to next cycle iteration
//
// Register() can be used for setting additional clock related options.
// Now() is expected to return the current time in line with the implemented clock behaviour.
// Current time must have the desired timezone set.
type Clock interface {
	Register(*Cron) []Option
	Now() time.Time
	Timer(time.Time) (timer <-chan struct{}, stop func())
	NopTimer() (timer <-chan struct{}, stop func())
}
