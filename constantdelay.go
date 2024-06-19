package cron

import (
	"fmt"
	"time"
)

// constantDelaySchedule represents a simple recurring duty cycle, e.g. "Every 5 minutes".
// It does not support jobs more frequent than once a second.
type constantDelaySchedule struct {
	Delay time.Duration
}

// every returns a crontab Schedule that activates once every duration.
// Delays that are less than on second or not a multiple of a second will return an error.
func every(duration time.Duration) (Schedule, error) {
	if duration < time.Second {
		return nil, fmt.Errorf("delay must be at least one second but was %s", duration.String())
	} else if duration%time.Second != 0 {
		return nil, fmt.Errorf("delay must be a multiple of one second but was %s", duration.String())
	}
	return constantDelaySchedule{Delay: duration}, nil
}

// Next returns the next time this should be run.
// This rounds so that the next activation time will be on the second.
func (schedule constantDelaySchedule) Next(t time.Time) time.Time {
	return t.Add(schedule.Delay - time.Duration(t.Nanosecond())*time.Nanosecond)
}
