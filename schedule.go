package cron

import (
	"math"
	"sync"
	"time"

	"github.com/gdgvda/cron/internal/matcher"
)

// Schedule describes a job's duty cycle.
type Schedule interface {
	// Next returns the next activation time, later than the given time.
	// Next is invoked initially, and then each time the job is run.
	Next(time.Time) time.Time
	MinInterval() time.Duration
}

// Specifies a duty cycle (to the second granularity), based on a
// traditional crontab specification.
type DefaultSchedule struct {
	secondMatch, minuteMatch, hourMatch, dayMatch, monthMatch matcher.Matcher

	// Override location for this schedule.
	location *time.Location

	// Constant delay mode when not zero
	delay time.Duration

	// minInterval computes MinInterval once and caches it. Copies made by
	// WithLocation share it, the location being ignored.
	minInterval func() time.Duration
}

func newDefaultSchedule(second, minute, hour, day, month matcher.Matcher, location *time.Location) *DefaultSchedule {
	s := &DefaultSchedule{
		secondMatch: second,
		minuteMatch: minute,
		hourMatch:   hour,
		dayMatch:    day,
		monthMatch:  month,
		location:    location,
	}
	s.minInterval = sync.OnceValue(s.computeMinInterval)
	return s
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
		t = startOfDay(year, month, 1, loc)
		if t.Year() != year {
			goto WRAP
		}
	}

	// Now get a day in that month.
	for year, month, day := t.Date(); !s.dayMatch(t); {
		day++
		t = startOfDay(year, month, day, loc)
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
func startOfDay(year int, month time.Month, day int, loc *time.Location) time.Time {
	t := time.Date(year, month, day, 0, 0, 0, 0, loc)
	_, offset := t.Zone()
	midnight := time.Date(year, month, day, 0, 0, 0, 0, time.UTC).Unix()
	// When midnight is skipped, time.Date returns a time on either side of the
	// change, depending on the offset.
	switch wall := t.Unix() + int64(offset); {
	case wall < midnight:
		_, t = t.ZoneBounds()
	case wall > midnight:
		t, _ = t.ZoneBounds()
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

// MinInterval returns the shortest time that can separate two consecutive
// activations of the schedule, or zero if the schedule never activates.
//
// All fields are taken into account, and the location is assumed to keep a
// constant UTC offset: the result is exact in UTC or any fixed-offset zone. In
// a location that observes daylight saving time, activations on either side of
// a clock change can be closer than this, down to one second apart.
//
// The result is computed on first call, which takes under a millisecond, and
// cached afterwards.
func (s *DefaultSchedule) MinInterval() time.Duration {
	if s.delay != 0 {
		return s.delay
	}
	return s.minInterval()
}

func (s *DefaultSchedule) computeMinInterval() time.Duration {
	days := s.minDayGap()
	if days == 0 {
		return 0
	}
	hours := minGap(s.hourMatch, 24, time.Hour, days)
	minutes := minGap(s.minuteMatch, 60, time.Minute, hours)
	seconds := minGap(s.secondMatch, 60, time.Second, minutes)
	return time.Duration(seconds) * time.Second
}

// gregorianCycle is the number of years after which the Gregorian calendar,
// days of the week included, repeats itself.
const gregorianCycle = 400

// dayMatches holds the days matched in a month or a year, as offsets from its
// first day. Gap is the smallest distance between two of them, math.MaxInt if
// there are fewer than two. First and last are -1 if there are none.
type dayMatches struct {
	first, last, gap int
}

var noDayMatches = dayMatches{-1, -1, math.MaxInt}

// extend appends next, starting offset days after the first day of m.
func (m dayMatches) extend(offset int, next dayMatches) dayMatches {
	if next.first < 0 {
		return m
	}
	if m.first < 0 {
		return dayMatches{offset + next.first, offset + next.last, next.gap}
	}
	return dayMatches{m.first, offset + next.last, min(m.gap, next.gap, offset+next.first-m.last)}
}

// monthLengths are the lengths of the months of a non-leap year: February
// gains a day in leap years.
var monthLengths = [12]int{31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}

// minDayGap returns the smallest number of days between two consecutive days
// matched by the month and day fields, or zero if no day is ever matched.
//
// Which days of a month are matched by the day fields only depends on its
// length and on the day of the week of its 1st: each of these 28 month types
// is scanned once, and the month field is checked once per month. Years are
// chained from their matching months, once for each of the 14 year types given
// by the day of the week of January 1st and leap or not. Years are in turn
// chained over a whole Gregorian cycle, so that gaps spanning several years,
// including around non-leap century years, are exact.
func (s *DefaultSchedule) minDayGap() int {
	var matchedMonths [12]bool
	for m := range matchedMonths {
		matchedMonths[m] = s.monthMatch(time.Date(2000, time.Month(m+1), 1, 0, 0, 0, 0, time.UTC))
	}
	var monthTypes [28]*dayMatches
	var yearTypes [14]*dayMatches
	days := noDayMatches
	offset := 0
	weekday := time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC).Weekday()
	for year := 2000; year < 2000+gregorianCycle; year++ {
		leap := year%4 == 0 && (year%100 != 0 || year%400 == 0)
		length := 365
		if leap {
			length++
		}
		c := int(weekday) + 7*(length-365)
		if yearTypes[c] == nil {
			yearTypes[c] = s.matchYear(year, weekday, leap, matchedMonths, &monthTypes)
		}
		days = days.extend(offset, *yearTypes[c])
		offset += length
		weekday = (weekday + time.Weekday(length%7)) % 7
	}
	if days.first < 0 {
		return 0
	}
	return min(days.gap, offset-days.last+days.first)
}

// matchYear chains the matched months of year, starting on weekday, scanning
// those of a type missing from monthTypes.
func (s *DefaultSchedule) matchYear(year int, weekday time.Weekday, leap bool, matchedMonths [12]bool, monthTypes *[28]*dayMatches) *dayMatches {
	days := noDayMatches
	offset := 0
	for m, length := range monthLengths {
		if m == 1 && leap {
			length++
		}
		if matchedMonths[m] {
			c := int(weekday) + 7*(length-28)
			if monthTypes[c] == nil {
				monthTypes[c] = s.matchMonth(time.Date(year, time.Month(m+1), 1, 0, 0, 0, 0, time.UTC), length)
			}
			days = days.extend(offset, *monthTypes[c])
		}
		offset += length
		weekday = (weekday + time.Weekday(length%7)) % 7
	}
	return &days
}

// matchMonth returns the days matched by the day fields in the month starting
// at first and lasting length days.
func (s *DefaultSchedule) matchMonth(first time.Time, length int) *dayMatches {
	days := noDayMatches
	for i := 0; i < length; i++ {
		if !s.dayMatch(first.Add(time.Duration(i) * 24 * time.Hour)) {
			continue
		}
		if days.first < 0 {
			days.first = i
		} else {
			days.gap = min(days.gap, i-days.last)
		}
		days.last = i
	}
	return &days
}

// minGap returns the smallest distance between two consecutive values matched
// by a field having size values, each lasting unit. The last value is followed
// by the first one after the enclosing field advanced by outerGap.
func minGap(match matcher.Matcher, size int, unit time.Duration, outerGap int) int {
	base := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	first, last := -1, -1
	gap := outerGap * size
	for i := 0; i < size; i++ {
		if !match(base.Add(time.Duration(i) * unit)) {
			continue
		}
		if first < 0 {
			first = i
		} else if i-last < gap {
			gap = i - last
		}
		last = i
	}
	if wrap := outerGap*size - last + first; wrap < gap {
		gap = wrap
	}
	return gap
}
