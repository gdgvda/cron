package cron

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestActivation(t *testing.T) {
	tests := []struct {
		time, spec string
		expected   bool
	}{
		// Every fifteen minutes.
		{"Mon Jul 9 15:00 2012", "0/15 * * * *", true},
		{"Mon Jul 9 15:45 2012", "0/15 * * * *", true},
		{"Mon Jul 9 15:40 2012", "0/15 * * * *", false},

		// Every fifteen minutes, starting at 5 minutes.
		{"Mon Jul 9 15:05 2012", "5/15 * * * *", true},
		{"Mon Jul 9 15:20 2012", "5/15 * * * *", true},
		{"Mon Jul 9 15:50 2012", "5/15 * * * *", true},

		// Named months
		{"Sun Jul 15 15:00 2012", "0/15 * * Jul *", true},
		{"Sun Jul 15 15:00 2012", "0/15 * * Jun *", false},

		// Everything set.
		{"Sun Jul 15 08:30 2012", "30 08 ? Jul Sun", true},
		{"Sun Jul 15 08:30 2012", "30 08 15 Jul ?", true},
		{"Mon Jul 16 08:30 2012", "30 08 ? Jul Sun", false},
		{"Mon Jul 16 08:30 2012", "30 08 15 Jul ?", false},

		// Predefined schedules
		{"Mon Jul 9 15:00 2012", "@hourly", true},
		{"Mon Jul 9 15:04 2012", "@hourly", false},
		{"Mon Jul 9 15:00 2012", "@daily", false},
		{"Mon Jul 9 00:00 2012", "@daily", true},
		{"Mon Jul 9 00:00 2012", "@weekly", false},
		{"Sun Jul 8 00:00 2012", "@weekly", true},
		{"Sun Jul 8 01:00 2012", "@weekly", false},
		{"Sun Jul 8 00:00 2012", "@monthly", false},
		{"Sun Jul 1 00:00 2012", "@monthly", true},

		// Test interaction of DOW and DOM.
		// If both are restricted, then only one needs to match.
		{"Sun Jul 15 00:00 2012", "* * 1,15 * Sun", true},
		{"Fri Jun 15 00:00 2012", "* * 1,15 * Sun", true},
		{"Wed Aug 1 00:00 2012", "* * 1,15 * Sun", true},
		{"Sun Jul 15 00:00 2012", "* * */10 * Sun", true}, // verifies #70

		// However, if one has a star, then both need to match.
		{"Sun Jul 15 00:00 2012", "* * * * Mon", false},
		{"Mon Jul 9 00:00 2012", "* * 1,15 * *", false},
		{"Sun Jul 15 00:00 2012", "* * 1,15 * *", true},
		{"Sun Jul 15 00:00 2012", "* * */2 * Sun", true},

		// Last day of month
		{"Sun Jul 15 00:00 2012", "* * L * *", false},
		{"Tue Jul 31 00:00 2012", "* * L * *", true},
		{"Tue Jul 31 00:00 2012", "* * L * Mon", true},
		{"Tue Feb 1 00:00 2012", "* * L-28 2 *", true},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("now=%s,spec=%s", test.time, strings.Replace(test.spec, "/", "|", -1)), func(t *testing.T) {
			sched, err := standardParser.Parse(test.spec)
			if err != nil {
				t.Fatal(err)
			}
			actual := sched.Next(getTime(test.time).Add(-1 * time.Second))
			expected := getTime(test.time)
			if test.expected && expected != actual || !test.expected && expected == actual {
				t.Fatalf("Fail evaluating %s on %s: (expected) %s != %s (actual)",
					test.spec, test.time, expected, actual)
			}
		})
	}
}

func TestNext(t *testing.T) {
	runs := []struct {
		time, spec string
		expected   string
	}{
		// Simple cases
		{"Mon Jul 9 14:45 2012", "0 0/15 * * * *", "Mon Jul 9 15:00 2012"},
		{"Mon Jul 9 14:59 2012", "0 0/15 * * * *", "Mon Jul 9 15:00 2012"},
		{"Mon Jul 9 14:59:59 2012", "0 0/15 * * * *", "Mon Jul 9 15:00 2012"},

		// Wrap around hours
		{"Mon Jul 9 15:45 2012", "0 20-35/15 * * * *", "Mon Jul 9 16:20 2012"},

		// Wrap around days
		{"Mon Jul 9 23:46 2012", "0 */15 * * * *", "Tue Jul 10 00:00 2012"},
		{"Mon Jul 9 23:45 2012", "0 20-35/15 * * * *", "Tue Jul 10 00:20 2012"},
		{"Mon Jul 9 23:35:51 2012", "15/35 20-35/15 * * * *", "Tue Jul 10 00:20:15 2012"},
		{"Mon Jul 9 23:35:51 2012", "15/35 20-35/15 1/2 * * *", "Tue Jul 10 01:20:15 2012"},
		{"Mon Jul 9 23:35:51 2012", "15/35 20-35/15 10-12 * * *", "Tue Jul 10 10:20:15 2012"},

		{"Mon Jul 9 23:35:51 2012", "15/35 20-35/15 1/2 */2 * *", "Thu Jul 11 01:20:15 2012"},
		{"Mon Jul 9 23:35:51 2012", "15/35 20-35/15 * 9-20 * *", "Wed Jul 10 00:20:15 2012"},
		{"Mon Jul 9 23:35:51 2012", "15/35 20-35/15 * 9-20 Jul *", "Wed Jul 10 00:20:15 2012"},

		// Wrap around months
		{"Mon Jul 9 23:35 2012", "0 0 0 9 Apr-Oct ?", "Thu Aug 9 00:00 2012"},
		{"Mon Jul 9 23:35 2012", "0 0 0 */5 Apr,Aug,Oct Mon", "Tue Aug 1 00:00 2012"},
		{"Mon Jul 9 23:35 2012", "0 0 0 */5 Oct Mon", "Mon Oct 1 00:00 2012"},

		// Wrap around years
		{"Mon Jul 9 23:35 2012", "0 0 0 * Feb Mon", "Mon Feb 4 00:00 2013"},
		{"Mon Jul 9 23:35 2012", "0 0 0 * Feb Mon/2", "Fri Feb 1 00:00 2013"},

		// Wrap around minute, hour, day, month, and year
		{"Mon Dec 31 23:59:45 2012", "0 * * * * *", "Tue Jan 1 00:00:00 2013"},

		// Leap year
		{"Mon Jul 9 23:35 2012", "0 0 0 29 Feb ?", "Mon Feb 29 00:00 2016"},

		// Daylight savings time 2am EST (-5) -> 3am EDT (-4)
		{"2012-03-11T00:00:00-0500", "TZ=America/New_York 0 30 2 11 Mar ?", "2013-03-11T02:30:00-0400"},

		// hourly job
		{"2012-03-11T00:00:00-0500", "TZ=America/New_York 0 0 * * * ?", "2012-03-11T01:00:00-0500"},
		{"2012-03-11T01:00:00-0500", "TZ=America/New_York 0 0 * * * ?", "2012-03-11T03:00:00-0400"},
		{"2012-03-11T03:00:00-0400", "TZ=America/New_York 0 0 * * * ?", "2012-03-11T04:00:00-0400"},
		{"2012-03-11T04:00:00-0400", "TZ=America/New_York 0 0 * * * ?", "2012-03-11T05:00:00-0400"},

		// hourly job using CRON_TZ
		{"2012-03-11T00:00:00-0500", "CRON_TZ=America/New_York 0 0 * * * ?", "2012-03-11T01:00:00-0500"},
		{"2012-03-11T01:00:00-0500", "CRON_TZ=America/New_York 0 0 * * * ?", "2012-03-11T03:00:00-0400"},
		{"2012-03-11T03:00:00-0400", "CRON_TZ=America/New_York 0 0 * * * ?", "2012-03-11T04:00:00-0400"},
		{"2012-03-11T04:00:00-0400", "CRON_TZ=America/New_York 0 0 * * * ?", "2012-03-11T05:00:00-0400"},

		// 1am nightly job
		{"2012-03-11T00:00:00-0500", "TZ=America/New_York 0 0 1 * * ?", "2012-03-11T01:00:00-0500"},
		{"2012-03-11T01:00:00-0500", "TZ=America/New_York 0 0 1 * * ?", "2012-03-12T01:00:00-0400"},

		// 2am nightly job (skipped)
		{"2012-03-11T00:00:00-0500", "TZ=America/New_York 0 0 2 * * ?", "2012-03-12T02:00:00-0400"},

		// Daylight savings time 2am EDT (-4) => 1am EST (-5)
		{"2012-11-04T00:00:00-0400", "TZ=America/New_York 0 30 2 04 Nov ?", "2012-11-04T02:30:00-0500"},
		{"2012-11-04T01:45:00-0400", "TZ=America/New_York 0 30 1 04 Nov ?", "2012-11-04T01:30:00-0500"},

		// hourly job
		{"2012-11-04T00:00:00-0400", "TZ=America/New_York 0 0 * * * ?", "2012-11-04T01:00:00-0400"},
		{"2012-11-04T01:00:00-0400", "TZ=America/New_York 0 0 * * * ?", "2012-11-04T01:00:00-0500"},
		{"2012-11-04T01:00:00-0500", "TZ=America/New_York 0 0 * * * ?", "2012-11-04T02:00:00-0500"},

		// 1am nightly job (runs twice)
		{"2012-11-04T00:00:00-0400", "TZ=America/New_York 0 0 1 * * ?", "2012-11-04T01:00:00-0400"},
		{"2012-11-04T01:00:00-0400", "TZ=America/New_York 0 0 1 * * ?", "2012-11-04T01:00:00-0500"},
		{"2012-11-04T01:00:00-0500", "TZ=America/New_York 0 0 1 * * ?", "2012-11-05T01:00:00-0500"},

		// 2am nightly job
		{"2012-11-04T00:00:00-0400", "TZ=America/New_York 0 0 2 * * ?", "2012-11-04T02:00:00-0500"},
		{"2012-11-04T02:00:00-0500", "TZ=America/New_York 0 0 2 * * ?", "2012-11-05T02:00:00-0500"},

		// 3am nightly job
		{"2012-11-04T00:00:00-0400", "TZ=America/New_York 0 0 3 * * ?", "2012-11-04T03:00:00-0500"},
		{"2012-11-04T03:00:00-0500", "TZ=America/New_York 0 0 3 * * ?", "2012-11-05T03:00:00-0500"},

		// hourly job
		{"TZ=America/New_York 2012-11-04T00:00:00-0400", "0 0 * * * ?", "2012-11-04T01:00:00-0400"},
		{"TZ=America/New_York 2012-11-04T01:00:00-0400", "0 0 * * * ?", "2012-11-04T01:00:00-0500"},
		{"TZ=America/New_York 2012-11-04T01:00:00-0500", "0 0 * * * ?", "2012-11-04T02:00:00-0500"},

		// 1am nightly job (runs twice)
		{"TZ=America/New_York 2012-11-04T00:00:00-0400", "0 0 1 * * ?", "2012-11-04T01:00:00-0400"},
		{"TZ=America/New_York 2012-11-04T01:00:00-0400", "0 0 1 * * ?", "2012-11-04T01:00:00-0500"},
		{"TZ=America/New_York 2012-11-04T01:00:00-0500", "0 0 1 * * ?", "2012-11-05T01:00:00-0500"},

		// 2am nightly job
		{"TZ=America/New_York 2012-11-04T00:00:00-0400", "0 0 2 * * ?", "2012-11-04T02:00:00-0500"},
		{"TZ=America/New_York 2012-11-04T02:00:00-0500", "0 0 2 * * ?", "2012-11-05T02:00:00-0500"},

		// 3am nightly job
		{"TZ=America/New_York 2012-11-04T00:00:00-0400", "0 0 3 * * ?", "2012-11-04T03:00:00-0500"},
		{"TZ=America/New_York 2012-11-04T03:00:00-0500", "0 0 3 * * ?", "2012-11-05T03:00:00-0500"},

		// Unsatisfiable
		{"Mon Jul 9 23:35 2012", "0 0 0 30 Feb ?", ""},
		{"Mon Jul 9 23:35 2012", "0 0 0 31 Apr ?", ""},

		// Monthly job
		{"TZ=America/New_York 2012-11-04T00:00:00-0400", "0 0 3 3 * ?", "2012-12-03T03:00:00-0500"},

		// Test the scenario of DST resulting in midnight not being a valid time.
		// https://github.com/robfig/cron/issues/157
		{"2018-10-17T05:00:00-0400", "TZ=America/Sao_Paulo 0 0 9 10 * ?", "2018-11-10T06:00:00-0500"},
		{"2018-02-14T05:00:00-0500", "TZ=America/Sao_Paulo 0 0 9 22 * ?", "2018-02-22T07:00:00-0500"},

		// Midnight skipped on Friday, east of Greenwich: 0am EET (+2) -> 1am EEST (+3)
		{"2026-04-23T05:00:00+0200", "TZ=Africa/Cairo 0 0 5 * * THU", "2026-04-30T05:00:00+0300"},
		{"2026-04-23T05:00:00+0200", "TZ=Africa/Cairo 0 0 5 * * FRI", "2026-04-24T05:00:00+0300"},

		// Midnight skipped on Sunday, west of Greenwich: 0am CST (-5) -> 1am CDT (-4)
		{"2026-03-07T12:00:00-0500", "TZ=America/Havana 0 0 12 * * SAT", "2026-03-14T12:00:00-0400"},

		// Half an hour: 2am (+1030) -> 2:30am (+11)
		{"2026-10-04T00:00:00+1030", "TZ=Australia/Lord_Howe 0 0 0 * * ?", "2026-10-05T00:00:00+1100"},

		// Not on the hour: 2:45am (+1245) -> 3:45am (+1345), and back
		{"2026-09-27T00:00:00+1245", "TZ=Pacific/Chatham 0 50 3 * * ?", "2026-09-27T03:50:00+1345"},
		{"2026-04-05T02:50:00+1345", "TZ=Pacific/Chatham 0 50 2 * * ?", "2026-04-05T02:50:00+1245"},

		// December 30th skipped: midnight (-10) -> midnight on the 31st (+14)
		{"2011-12-29T12:00:00-1000", "TZ=Pacific/Apia 0 0 12 * * ?", "2011-12-31T12:00:00+1400"},
		{"2011-12-01T00:00:00-1000", "TZ=Pacific/Apia 0 0 12 30 12 ?", "2012-12-30T12:00:00+1400"},

		// Last day of month
		{"Tue Jan 2 23:35 2024", "0 0 0 L * *", "Wed Jan 31 00:00 2024"},
		{"Fri Feb 2 23:35 2024", "0 0 0 L * *", "Thu Feb 29 00:00 2024"},
		{"Thu Feb 2 23:35 2023", "0 0 0 L * *", "Tue Feb 28 00:00 2023"},
		{"Sat Nov 2 23:35 2024", "0 0 0 L * *", "Sat Nov 30 00:00 2024"},
		{"Sat Nov 30 18:25 2024", "0 20 18 L * *", "Tue Dec 31 18:20 2024"},
		{"Sat Nov 30 18:25 2024", "0 20 18 L-1 * *", "Tue Dec 30 18:20 2024"},
		{"Sat Nov 30 18:25 2024", "0 20 18 L-2 * *", "Tue Dec 29 18:20 2024"},
		{"Tue Jan 2 23:35 2024", "0 0 0 L,10 * *", "Wed Jan 10 00:00 2024"},
		{"Tue Jan 11 23:35 2024", "0 0 0 L,10 * *", "Wed Jan 31 00:00 2024"},
	}

	for _, c := range runs {
		t.Run(fmt.Sprintf("now=%s,spec=%s", c.time, strings.Replace(c.spec, "/", "|", -1)), func(t *testing.T) {
			sched, err := secondParser.Parse(c.spec)
			if err != nil {
				t.Fatal(err)
			}
			actual := sched.Next(getTime(c.time))
			expected := getTime(c.expected)
			if !actual.Equal(expected) {
				t.Fatalf("%s, \"%s\": (expected) %v != %v (actual)", c.time, c.spec, expected, actual)
			}
		})
	}
}

func TestErrors(t *testing.T) {
	invalidSpecs := []string{
		"xyz",
		"60 0 * * *",
		"0 60 * * *",
		"0 0 * * XYZ",
	}
	for _, spec := range invalidSpecs {
		t.Run(strings.Replace(spec, "/", "|", -1), func(t *testing.T) {
			_, err := standardParser.Parse(spec)
			if err == nil {
				t.Fatal("expected an error parsing: ", spec)
			}
		})
	}
}

func getTime(value string) time.Time {
	if value == "" {
		return time.Time{}
	}

	var location = time.Local
	if strings.HasPrefix(value, "TZ=") {
		parts := strings.Fields(value)
		loc, err := time.LoadLocation(parts[0][len("TZ="):])
		if err != nil {
			panic("could not parse location:" + err.Error())
		}
		location = loc
		value = parts[1]
	}

	var layouts = []string{
		"Mon Jan 2 15:04 2006",
		"Mon Jan 2 15:04:05 2006",
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, value, location); err == nil {
			return t
		}
	}
	if t, err := time.ParseInLocation("2006-01-02T15:04:05-0700", value, location); err == nil {
		return t
	}
	panic("could not parse time value " + value)
}

func TestNextWithTz(t *testing.T) {
	runs := []struct {
		time, spec string
		expected   string
	}{
		// Failing tests
		{"2016-01-03T13:09:03+0530", "14 14 * * *", "2016-01-03T14:14:00+0530"},
		{"2016-01-03T04:09:03+0530", "14 14 * * ?", "2016-01-03T14:14:00+0530"},

		// Passing tests
		{"2016-01-03T14:09:03+0530", "14 14 * * *", "2016-01-03T14:14:00+0530"},
		{"2016-01-03T14:00:00+0530", "14 14 * * ?", "2016-01-03T14:14:00+0530"},
	}
	for _, c := range runs {
		t.Run(fmt.Sprintf("now=%s,spec=%s", c.time, strings.Replace(c.spec, "/", "|", -1)), func(t *testing.T) {
			sched, err := standardParser.Parse(c.spec)
			if err != nil {
				t.Fatal(err)
			}
			actual := sched.Next(getTimeTZ(c.time))
			expected := getTimeTZ(c.expected)
			if !actual.Equal(expected) {
				t.Fatalf("%s, \"%s\": (expected) %v != %v (actual)", c.time, c.spec, expected, actual)
			}
		})
	}
}

func getTimeTZ(value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	t, err := time.Parse("Mon Jan 2 15:04 2006", value)
	if err != nil {
		t, err = time.Parse("Mon Jan 2 15:04:05 2006", value)
		if err != nil {
			t, err = time.Parse("2006-01-02T15:04:05-0700", value)
			if err != nil {
				panic(err)
			}
		}
	}

	return t
}

// TestNextClockChanges checks Next over a day on either side of each clock
// change of a year against a walk over every minute, activating at each
// instant whose local time matches the schedule: twice in repeated hours and
// never in skipped ones.
func TestNextClockChanges(t *testing.T) {
	locations := []struct {
		name string
		year int
	}{
		{"America/New_York", 2026},
		{"Europe/Paris", 2026},
		{"Australia/Sydney", 2026},
		// By half an hour.
		{"Australia/Lord_Howe", 2026},
		// At 2:45 and 3:45.
		{"Pacific/Chatham", 2026},
		// At midnight, east and west of Greenwich.
		{"Africa/Cairo", 2026},
		{"Asia/Tehran", 2021},
		{"America/Havana", 2026},
		{"America/Santiago", 2026},
		{"America/Sao_Paulo", 2018},
		// Twice in a row during Ramadan.
		{"Africa/Casablanca", 2026},
		// Skipping December 30th.
		{"Pacific/Apia", 2011},
	}
	specs := []string{
		"0 * * * * *", "*/20 * 0-3 * * *", "0 0 * * * *", "0 30 * * * *",
		"0 0 0 * * *", "0 15 0 * * *", "0 0,30 0-3 * * *", "0 45 1,2 * * *",
		"0 50 2,3 * * *", "0 0 23 * * *", "0 30 23 * * *", "59 59 23 * * *",
		"0 0 5 * * THU", "0 0 5 * * FRI", "0 0 12 * * SAT", "0 0 0 * * SUN",
		"0 0 12 30 * *", "0 0 12 31 * *",
	}
	for _, l := range locations {
		loc, err := time.LoadLocation(l.name)
		if err != nil {
			t.Fatal(err)
		}
		changes := offsetChanges(loc, time.Date(l.year, time.January, 1, 0, 0, 0, 0, loc), time.Date(l.year+1, time.January, 1, 0, 0, 0, 0, loc))
		if len(changes) == 0 {
			t.Fatalf("%s: no clock change in %d", l.name, l.year)
		}
		for _, spec := range specs {
			sched, err := secondParser.Parse(spec)
			if err != nil {
				t.Fatal(err)
			}
			sched = sched.WithLocation(loc)
			for _, change := range changes {
				from, to := change.Add(-24*time.Hour), change.Add(24*time.Hour)
				expected := activationsBetween(sched, from, to)
				var actual []time.Time
				for next := sched.Next(from.Add(-time.Second)); next.Before(to); next = sched.Next(next) {
					actual = append(actual, next)
				}
				if i := firstDifference(expected, actual); i >= 0 {
					t.Errorf("%s %s, around %v: activation %d: (expected) %v != %v (actual)",
						l.name, spec, change.In(loc), i, at(expected, i), at(actual, i))
				}
			}
		}
	}
}

// offsetChanges returns the instants between from and to at which the UTC
// offset of loc changes.
func offsetChanges(loc *time.Location, from, to time.Time) []time.Time {
	var changes []time.Time
	for t := from.In(loc); ; {
		_, end := t.ZoneBounds()
		if end.IsZero() || !end.Before(to) {
			return changes
		}
		_, before := t.Zone()
		if _, after := end.Zone(); after != before {
			changes = append(changes, end)
		}
		t = end
	}
}

// activationsBetween returns the activations of s from from, included, to to,
// excluded, by checking the local time of every minute, from aligned to a
// minute.
func activationsBetween(s *DefaultSchedule, from, to time.Time) []time.Time {
	var activations []time.Time
	for u := from.In(s.location); u.Before(to); u = u.Add(time.Minute) {
		if !s.monthMatch(u) || !s.dayMatch(u) || !s.hourMatch(u) || !s.minuteMatch(u) {
			continue
		}
		for second := 0; second < 60; second++ {
			if v := u.Add(time.Duration(second) * time.Second); s.secondMatch(v) {
				activations = append(activations, v)
			}
		}
	}
	return activations
}

// firstDifference returns the index of the first difference between a and b,
// or -1 if they are equal.
func firstDifference(a, b []time.Time) int {
	for i := 0; i < max(len(a), len(b)); i++ {
		if i >= len(a) || i >= len(b) || !a[i].Equal(b[i]) {
			return i
		}
	}
	return -1
}

func at(times []time.Time, i int) any {
	if i < len(times) {
		return times[i]
	}
	return "none"
}

// https://github.com/robfig/cron/issues/144
func TestSlash0NoHang(t *testing.T) {
	schedule := "TZ=America/New_York 15/0 * * * *"
	_, err := standardParser.Parse(schedule)
	if err == nil {
		t.Fatal("expected an error on 0 increment")
	}
}

func TestWithLocation(t *testing.T) {
	sched, err := secondParser.Parse("CRON_TZ=UTC 0 0 12 * * ?")
	if err != nil {
		t.Fatal(err)
	}

	now := time.Date(2012, 7, 9, 0, 0, 0, 0, time.UTC)

	next1 := sched.Next(now)
	if expected := time.Date(2012, 7, 9, 12, 0, 0, 0, time.UTC); !next1.Equal(expected) {
		t.Fatalf("expected %v, got %v", expected, next1)
	}

	sched2 := sched.WithLocation(time.FixedZone("UTC+2", 2*60*60))
	next2 := sched2.Next(now)
	if expected := time.Date(2012, 7, 9, 10, 0, 0, 0, time.UTC); !next2.Equal(expected) {
		t.Fatalf("expected %v, got %v", expected, next2)
	}
}

func TestWithLocationNil(t *testing.T) {
	sched, err := secondParser.Parse("0 0 12 * * ?")
	if err != nil {
		t.Fatal(err)
	}

	sched2 := sched.WithLocation(nil)
	if sched2 != sched {
		t.Fatalf("expected nil location to return same schedule")
	}
}

func TestMinInterval(t *testing.T) {
	tests := []struct {
		spec     string
		expected time.Duration
	}{
		// Constant delay
		{"@every 4s", 4 * time.Second},
		{"@every 1h30m", 90 * time.Minute},

		// Within a minute
		{"* * * * * *", time.Second},
		{"*/5 * * * * *", 5 * time.Second},
		{"0,3 0 0 29 2 *", 3 * time.Second},
		{"10,20-22/2,40 * * * * *", 2 * time.Second},

		// Across minutes
		{"0,57 * * * * *", 3 * time.Second},
		{"0,57 10,11 * * * *", 3 * time.Second},
		{"0,57 */2 * * * *", 57 * time.Second},
		{"0,30 */2 * * * *", 30 * time.Second},
		{"0 * * * * *", time.Minute},
		{"0 */15 * * * *", 15 * time.Minute},
		{"0 10,50 * * * *", 20 * time.Minute},

		// Across hours
		{"0,57 59,0 * * * *", 3 * time.Second},
		{"0,57 59,0 */2 * * *", 57 * time.Second},
		{"57 59,0 */2 * * *", 59 * time.Minute},
		{"@hourly", time.Hour},
		{"0 30 8,20 * * *", 12 * time.Hour},

		// Across days, assumed to be consecutive
		{"0,57 59,0 23,0 * * *", 3 * time.Second},
		{"0 0 22,2 * * *", 4 * time.Hour},
		{"@daily", 24 * time.Hour},
		{"@weekly", 24 * time.Hour},
		{"TZ=Asia/Tokyo 0 30 04 * * *", 24 * time.Hour},
	}

	for _, test := range tests {
		sched, err := secondParser.Parse(test.spec)
		if err != nil {
			t.Error(err)
			continue
		}
		actual := sched.MinInterval()
		if actual != test.expected {
			t.Errorf("%s: (expected) %v != %v (actual)", test.spec, test.expected, actual)
		}
	}
}
