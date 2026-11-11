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

	// minInterval computes MinInterval once and caches it. WithLocation gives
	// copies their own.
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
	s2.minInterval = sync.OnceValue(s2.computeMinInterval)
	return s2
}

// MinInterval returns the shortest time that can separate two consecutive
// activations of the schedule, or zero if the schedule never activates.
//
// All fields are taken into account, as well as the clock changes of the
// schedule's location: activations on either side of a change can be closer
// than usual, and those falling in an hour that is repeated occur twice. The
// changes of the coming years are taken from the time zone database, and
// assumed to recur in later years starting on the same day of the week and
// leap alike, as they do unless the rules change. A schedule without a
// location is assumed to run in [time.Local].
//
// The result is computed on first call, which takes about a millisecond, and
// cached afterwards.
func (s *DefaultSchedule) MinInterval() time.Duration {
	if s.delay != 0 {
		return s.delay
	}
	return s.minInterval()
}

// computeMinInterval chains the years of a Gregorian cycle from the current
// one, splitting each of them at the clock changes of the location, so that
// activations are positioned in actual time, in seconds from the start of the
// cycle. The cycle is assumed to repeat.
func (s *DefaultSchedule) computeMinInterval() time.Duration {
	a := newActivations(s)
	start := time.Now().In(s.location).Year()
	m, length := noMatches, int64(0)
	for year := start; year < start+gregorianCycle; year++ {
		y := a.year(year)
		m = m.extend(length, y.matches)
		length += y.length
	}
	if m.first < 0 {
		return 0
	}
	return time.Duration(min(m.gap, length-m.last+m.first)) * time.Second
}

// clockChange is a change of the UTC offset of a location, the clock going
// from and to local times, in seconds from the start of the year: forward if
// to is after from, back otherwise.
type clockChange struct {
	from, to int64
}

// clockChanges returns the changes of the UTC offset of loc during year.
func clockChanges(loc *time.Location, year int) []clockChange {
	var changes []clockChange
	start := yearStart(year)
	end := time.Date(year+1, time.January, 1, 0, 0, 0, 0, loc)
	for t := time.Date(year, time.January, 1, 0, 0, 0, 0, loc); ; {
		_, next := t.ZoneBounds()
		if next.IsZero() || !next.Before(end) {
			return changes
		}
		if !next.After(t) {
			// Past the transitions listed in the zone file (after 2037
			// with "fat" files, after the last rule change with "slim"
			// ones like Go's), Go applies the yearly rule and places
			// year ends one day early in leap years.
			next = t.Add(24 * time.Hour)
		}
		// Periods can also end without a change of offset.
		_, from := t.Zone()
		if _, to := next.Zone(); to != from {
			changes = append(changes, clockChange{next.Unix() + int64(from) - start, next.Unix() + int64(to) - start})
		}
		t = next
	}
}

const (
	minuteSeconds = 60
	hourSeconds   = 60 * minuteSeconds
	daySeconds    = 24 * hourSeconds
)

// gregorianCycle is the number of years after which the Gregorian calendar,
// days of the week included, repeats itself.
const gregorianCycle = 400

// matches holds activations within a span of time, as offsets from its start.
// Gap is the smallest distance between two consecutive ones, math.MaxInt64 if
// there are fewer than two. First and last are -1 if there are none.
type matches struct {
	first, last, gap int64
}

var (
	noMatches = matches{-1, -1, math.MaxInt64}
	oneMatch  = matches{0, 0, math.MaxInt64}
)

// extend appends next, starting offset after the start of m.
func (m matches) extend(offset int64, next matches) matches {
	if next.first < 0 {
		return m
	}
	if m.first < 0 {
		return matches{offset + next.first, offset + next.last, next.gap}
	}
	return matches{m.first, offset + next.last, min(m.gap, next.gap, offset+next.first-m.last)}
}

// monthLengths are the lengths of the months of a non-leap year: February
// gains a day in leap years.
var monthLengths = [12]int{31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}

// monthType holds the days matched by the day fields in a month, as bits from
// the lowest for its 1st, and the activations over the whole month.
type monthType struct {
	days    uint32
	matches matches
}

// yearType holds the activations over a year, and its length in seconds once
// clock changes are accounted for.
type yearType struct {
	matches matches
	length  int64
}

// activations computes the activations of a schedule within spans of local
// time, as offsets in seconds from their start.
//
// Which days of a month are matched by the day fields only depends on its
// length and on the day of the week of its 1st: each of these 28 month types
// is scanned once, and the month field is checked once per month. A whole year
// only depends on the day of the week of January 1st and being leap or not,
// clock changes included: each of these 14 year types is chained once from its
// matching months, split at the clock changes of the first year of the type.
type activations struct {
	s                *DefaultSchedule
	matchedMonths    [12]bool
	hours            [24]bool
	minutes, seconds [60]bool
	hour, day        matches
	monthTypes       [28]*monthType
	yearTypes        [14]*yearType
}

func newActivations(s *DefaultSchedule) *activations {
	a := &activations{s: s}
	base := time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC)
	for i := range a.matchedMonths {
		a.matchedMonths[i] = s.monthMatch(base.AddDate(0, i, 0))
	}
	for i := range a.hours {
		a.hours[i] = s.hourMatch(base.Add(time.Duration(i) * time.Hour))
	}
	for i := range a.minutes {
		a.minutes[i] = s.minuteMatch(base.Add(time.Duration(i) * time.Minute))
		a.seconds[i] = s.secondMatch(base.Add(time.Duration(i) * time.Second))
	}
	minute := repeat(a.seconds[:], 1, oneMatch)
	a.hour = repeat(a.minutes[:], minuteSeconds, minute)
	a.day = repeat(a.hours[:], hourSeconds, a.hour)
	return a
}

// repeat chains inner at each of the matched values of a field, each lasting
// unit seconds.
func repeat(matched []bool, unit int64, inner matches) matches {
	m := noMatches
	for i, ok := range matched {
		if ok {
			m = m.extend(int64(i)*unit, inner)
		}
	}
	return m
}

// year returns the activations over the whole year, chaining the local times
// the clock goes through between its changes.
func (a *activations) year(year int) yearType {
	c := int(time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC).Weekday())
	if isLeap(year) {
		c += 7
	}
	if a.yearTypes[c] != nil {
		return *a.yearTypes[c]
	}
	length := yearStart(year+1) - yearStart(year)
	changes := append(clockChanges(a.s.location, year), clockChange{length, length})
	y, from := yearType{noMatches, 0}, int64(0)
	for _, change := range changes {
		to := min(max(from, change.from), length)
		y.matches = y.matches.extend(y.length, a.months(year, from, to))
		y.length += to - from
		from = min(max(0, change.to), length)
	}
	a.yearTypes[c] = &y
	return y
}

// months returns the activations between lo and hi seconds from the start of
// the year.
func (a *activations) months(year int, lo, hi int64) matches {
	m := noMatches
	start := int64(0)
	weekday := time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC).Weekday()
	for i, length := range monthLengths {
		if i == 1 && isLeap(year) {
			length++
		}
		end := start + int64(length)*daySeconds
		if a.matchedMonths[i] && start < hi && lo < end {
			c := int(weekday) + 7*(length-28)
			if a.monthTypes[c] == nil {
				a.monthTypes[c] = a.monthType(time.Date(year, time.Month(i+1), 1, 0, 0, 0, 0, time.UTC), length)
			}
			if lo <= start && end <= hi {
				m = m.extend(start-lo, a.monthTypes[c].matches)
			} else {
				from := max(lo, start)
				m = m.extend(from-lo, a.days(a.monthTypes[c].days, from-start, min(hi, end)-start))
			}
		}
		start = end
		weekday = (weekday + time.Weekday(length%7)) % 7
	}
	return m
}

// monthType scans the month starting at first and lasting length days.
func (a *activations) monthType(first time.Time, length int) *monthType {
	var days uint32
	for i := 0; i < length; i++ {
		if a.s.dayMatch(first.Add(time.Duration(i) * 24 * time.Hour)) {
			days |= 1 << i
		}
	}
	return &monthType{days, a.days(days, 0, int64(length)*daySeconds)}
}

// days returns the activations of the given days of a month, between lo and hi
// seconds from its start.
func (a *activations) days(days uint32, lo, hi int64) matches {
	m := noMatches
	for i := lo / daySeconds; i*daySeconds < hi; i++ {
		if days&(1<<i) == 0 {
			continue
		}
		start := i * daySeconds
		from := max(lo, start)
		m = m.extend(from-lo, a.timesOfDay(from-start, min(hi, start+daySeconds)-start))
	}
	return m
}

// timesOfDay returns the activations of a matched day between lo and hi
// seconds from its start.
func (a *activations) timesOfDay(lo, hi int64) matches {
	if lo == 0 && hi == daySeconds {
		return a.day
	}
	m := noMatches
	for i := lo / hourSeconds; i*hourSeconds < hi; i++ {
		if !a.hours[i] {
			continue
		}
		start, end := i*hourSeconds, (i+1)*hourSeconds
		if lo <= start && end <= hi {
			m = m.extend(start-lo, a.hour)
			continue
		}
		for t := max(lo, start); t < min(hi, end); t++ {
			if a.minutes[t/minuteSeconds%60] && a.seconds[t%minuteSeconds] {
				m = m.extend(t-lo, oneMatch)
			}
		}
	}
	return m
}

// yearStart returns the start of year in seconds since the Unix epoch, in UTC.
func yearStart(year int) int64 {
	return time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC).Unix()
}

func isLeap(year int) bool {
	return year%4 == 0 && (year%100 != 0 || year%400 == 0)
}
