package cron

import (
	"testing"
	"time"
)

func TestConstantDelayNext(t *testing.T) {
	tests := []struct {
		time     string
		delay    time.Duration
		expected string
	}{
		// Simple cases
		{"Mon Jul 9 14:45 2012", 15 * time.Minute, "Mon Jul 9 15:00 2012"},
		{"Mon Jul 9 14:59 2012", 15 * time.Minute, "Mon Jul 9 15:14 2012"},
		{"Mon Jul 9 14:59:59 2012", 15 * time.Minute, "Mon Jul 9 15:14:59 2012"},

		// Wrap around hours
		{"Mon Jul 9 15:45 2012", 35 * time.Minute, "Mon Jul 9 16:20 2012"},

		// Wrap around days
		{"Mon Jul 9 23:46 2012", 14 * time.Minute, "Tue Jul 10 00:00 2012"},
		{"Mon Jul 9 23:45 2012", 35 * time.Minute, "Tue Jul 10 00:20 2012"},
		{"Mon Jul 9 23:35:51 2012", 44*time.Minute + 24*time.Second, "Tue Jul 10 00:20:15 2012"},
		{"Mon Jul 9 23:35:51 2012", 25*time.Hour + 44*time.Minute + 24*time.Second, "Thu Jul 11 01:20:15 2012"},

		// Wrap around months
		{"Mon Jul 9 23:35 2012", 91*24*time.Hour + 25*time.Minute, "Thu Oct 9 00:00 2012"},

		// Wrap around minute, hour, day, month, and year
		{"Mon Dec 31 23:59:45 2012", 15 * time.Second, "Tue Jan 1 00:00:00 2013"},

		// Round to nearest second when calculating the next time.
		{"Mon Jul 9 14:45:00.005 2012", 15 * time.Minute, "Mon Jul 9 15:00 2012"},
	}

	for _, c := range tests {
		every, err := every(c.delay)
		if err != nil {
			t.Errorf("every(%s) returned error: %v", c.delay, err)
		}
		actual := every.Next(getTime(c.time))
		expected := getTime(c.expected)
		if actual != expected {
			t.Errorf("%s, \"%s\": (expected) %v != %v (actual)", c.time, c.delay, expected, actual)
		}
	}
}

func TestConstantDelayErrors(t *testing.T) {
	tests := []struct {
		delay time.Duration
		error bool
	}{
		{0, true},
		{time.Nanosecond, true},
		{time.Microsecond, true},
		{time.Millisecond, true},
		{time.Second, false},
		{time.Minute, false},
		{time.Hour, false},
		{24 * time.Hour, false},
		{7 * 24 * time.Hour, false},
		{30 * 24 * time.Hour, false},
		{365 * 24 * time.Hour, false},
	}

	for _, c := range tests {
		_, err := every(c.delay)
		if err != nil && !c.error {
			t.Errorf("every(%s) returned error: %v", c.delay, err)
		} else if err == nil && c.error {
			t.Errorf("every(%s) did not return error", c.delay)
		}
	}
}
