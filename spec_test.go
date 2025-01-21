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
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("now=%s,spec=%s", test.time, strings.Replace(test.spec, "/", "|", -1)), func(t *testing.T) {
			sched, err := ParseStandard(test.spec)
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
			_, err := ParseStandard(spec)
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
			sched, err := ParseStandard(c.spec)
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

// https://github.com/robfig/cron/issues/144
func TestSlash0NoHang(t *testing.T) {
	schedule := "TZ=America/New_York 15/0 * * * *"
	_, err := ParseStandard(schedule)
	if err == nil {
		t.Fatal("expected an error on 0 increment")
	}
}
