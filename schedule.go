package cron

import (
	"time"

	"github.com/gdgvda/cron/internal/matcher"
)

// Schedule describes a job's duty cycle.
type Schedule interface {
	// Next returns the next activation time, later than the given time.
	// Next is invoked initially, and then each time the job is run.
	Next(time.Time) time.Time
}

// Specifies a duty cycle (to the second granularity), based on a
// traditional crontab specification.
type DefaultSchedule struct {
	secondMatch, minuteMatch, hourMatch, dayMatch, monthMatch matcher.Matcher

	// Override location for this schedule.
	location *time.Location

	// Constant delay mode when not zero
	delay time.Duration
}

// Next returns the next time this schedule is activated, greater than the given
// time.  If no time can be found to satisfy the schedule, return the zero time.
//
// Local times repeated by a clock change of the schedule's location activate
// it twice, and those skipped by one do not activate it.
func (s *DefaultSchedule) Next(t time.Time) time.Time {
	if s.delay != 0 {
		return t.Add(s.delay - time.Duration(t.Nanosecond())*time.Nanosecond)
	}

	// General approach
	//
	// For Month, Day, Hour, Minute, Second:
	// Check if the time value matches.  If yes, continue to the next field.
	// If the field doesn't match the schedule, then move to the start of its
	// next value until it matches. When this changes the previous field, go
	// back to the beginning of the field list (since it is necessary to
	// re-verify previous field values).
	//
	// Moves are made in actual time, so that local times repeated by a clock
	// change are visited twice. The start of a day is the clock change that
	// skips midnight if any, and moves to the next hour stop at clock changes,
	// as the hour can change there at any minute. Clock changes are assumed to
	// happen on the minute, as they all have since 1972.

	// Convert the given time into the schedule's timezone, if one is specified.
	// Save the original timezone so we can convert back after we find a time.
	// Note that schedules without a time zone specified (time.Local) are treated
	// as local to the time provided.
	origLocation := t.Location()
	loc := s.location
	if loc == time.Local {
		loc = t.Location()
	}
	if s.location != time.Local {
		t = t.In(s.location)
	}

	// Start at the earliest possible time (the upcoming second).
	t = t.Add(1*time.Second - time.Duration(t.Nanosecond())*time.Nanosecond)

	// If no time is found within five years, return zero.
	yearLimit := t.Year() + 5

WRAP:
	if t.Year() > yearLimit {
		return time.Time{}
	}

	// Find the first applicable month.
	// If it's this month, then do nothing.
	for year, month := t.Year(), t.Month(); !s.monthMatch(t); {
		month++
		t = startOfDay(year, month, 1, loc, t)
		if t.Year() != year {
			goto WRAP
		}
	}

	// Now get a day in that month.
	for year, month, day := t.Date(); !s.dayMatch(t); {
		day++
		t = startOfDay(year, month, day, loc, t)
		if t.Month() != month {
			goto WRAP
		}
	}

	for day := t.Day(); !s.hourMatch(t); {
		t = nextHour(t)
		if t.Day() != day {
			goto WRAP
		}
	}

	for hour := t.Hour(); !s.minuteMatch(t); {
		t = t.Add(time.Duration(minuteSeconds-t.Second()) * time.Second)
		if t.Hour() != hour {
			goto WRAP
		}
	}

	for minute := t.Minute(); !s.secondMatch(t); {
		t = t.Add(1 * time.Second)
		if t.Minute() != minute {
			goto WRAP
		}
	}

	return t.In(origLocation)
}

// startOfDay returns the first instant of a day in loc, normalized like
// time.Date: midnight, or the clock change skipping it, which starts the next
// day if the whole day is skipped.
func startOfDay(year int, month time.Month, day int, loc *time.Location, notBefore time.Time) time.Time {
	t := time.Date(year, month, day, 0, 0, 0, 0, loc)
	_, offset := t.Zone()
	midnight := time.Date(year, month, day, 0, 0, 0, 0, time.UTC).Unix()
	// When midnight is skipped, time.Date returns a time on either side of the
	// change, depending on the offset.
	switch wall := t.Unix() + int64(offset); {
	case wall < midnight:
		_, t = t.ZoneBounds()
		return t
	case wall > midnight:
		t, _ = t.ZoneBounds()
		return t
	}
	// Midnight can occur twice when the clock is set back across it, and
	// time.Date returns either occurrence: take the first one not before
	// notBefore.
	start, end := t.ZoneBounds()
	if !start.IsZero() {
		// detect if change earlier than `t` midnight occurence did set back clock
		if _, prev := start.Add(-time.Second).Zone(); prev > offset {
			// enough back to move midnight before start => `t` is 2nd occurence
			if earlier := t.Add(time.Duration(offset-prev) * time.Second); earlier.Before(start) && !earlier.Before(notBefore) {
				// return 1st occurence if not past notBefore
				return earlier
			}
		}
	}
	if !end.IsZero() && t.Before(notBefore) {
		// `t` is before notBefore and is assumed to be 1st midnight occurrence
		// detect if change later than 1st midnight occurence would set back clock
		if _, next := end.Zone(); next < offset {
			// enough back to move midnight before end => `t` is indeed first occurence
			if later := t.Add(time.Duration(offset-next) * time.Second); !later.Before(end) {
				// return 2nd occurence
				return later
			}
		}
	}
	return t
}

// nextHour returns the start of the hour after the one of t, in its location,
// or the clock change ending the hour if it comes first.
func nextHour(t time.Time) time.Time {
	_, minute, second := t.Clock()
	next := t.Add(time.Duration(hourSeconds-minute*minuteSeconds-second) * time.Second)
	if _, end := t.ZoneBounds(); end.After(t) && end.Before(next) {
		return end
	}
	return next
}

// WithLocation returns a copy of the schedule with the given location.
// If the location is nil, it returns the original schedule.
func (s *DefaultSchedule) WithLocation(l *time.Location) *DefaultSchedule {
	if l == nil {
		return s
	}
	s2 := new(DefaultSchedule)
	*s2 = *s
	s2.location = l
	return s2
}
