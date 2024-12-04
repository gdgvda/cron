package parser

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestParseHourMatcher(t *testing.T) {
	tests := []struct {
		spec     string
		time     string
		expected bool
	}{
		{"*", "21:00:21", true},
		{"*", "23:04:59", true},

		{"23", "23:00:23", true},
		{"7", "07:17:23", true},
		{"23", "22:00:24", false},

		{"11-12", "10:00:10", false},
		{"11-12", "11:00:15", true},
		{"11-12", "12:00:15", true},
		{"11-12", "13:00:17", false},
		{"0-23", "15:17:23", true},

		{"0/5", "00:00:00", true},
		{"0/5", "05:00:15", true},
		{"0/5", "07:00:40", false},
		{"*/5", "00:00:00", true},
		{"*/5", "05:00:45", true},
		{"*/5", "07:00:40", false},

		{"5/5", "05:00:15", true},
		{"5/5", "15:00:15", true},
		{"5/5", "20:00:23", true},
		{"5/5", "22:03:12", false},

		{"5-17/5", "05:00:15", true},
		{"5-17/5", "10:00:20", true},
		{"5-17/5", "20:00:50", false},

		{"5-17/5,20", "20:00:50", true},
		{"5-17/5,19", "18:00:50", false},

		{"1,2,3", "00:00:00", false},
		{"1,2,3", "02:16:02", true},
		{"1,02,3", "02:16:02", true},
	}

	const layout = "15:04:05"
	for _, test := range tests {
		t.Run(fmt.Sprintf("spec=%s,now=%s", strings.Replace(test.spec, "/", "|", -1), test.time), func(t *testing.T) {
			time, err := time.Parse(layout, test.time)
			if err != nil {
				t.Fatalf("expected nil error, got %s", err)
			}

			matcher, err := ParseHour(test.spec)
			if err != nil {
				t.Fatalf("expected nil error, got %s", err)
			}

			actual := matcher(time)
			if actual != test.expected {
				t.Fatalf("spec=%s, time=%s, expected=%t, got=%t",
					test.spec, test.time, test.expected, actual)
			}
		})
	}
}

func TestParseHourErrors(t *testing.T) {
	tests := []struct {
		spec     string
		expected string
	}{
		{"#", "failed to parse"},
		{"?", "failed to parse"},
		{"*-5", "invalid expression"},
		{"*-5/2", "invalid expression"},
		{"5-*", "failed to parse"},
		{"5-*/22", "failed to parse"},
		{"-1", "failed to parse"},
		{"-22", "failed to parse"},
		{"24", "value 24 out of valid range [0, 23]"},
		{"22-24", "value 24 out of valid range [0, 23]"},
		{"22-25/33", "value 25 out of valid range [0, 23]"},
		{"*//2", "invalid expression"},
		{"*/2/", "invalid expression"},
		{"*-2-", "invalid expression"},
		{"2-", "failed to parse"},
		{"1-2-", "invalid expression"},
	}

	for _, test := range tests {
		t.Run(strings.Replace(test.spec, "/", "|", -1), func(t *testing.T) {
			_, err := ParseHour(test.spec)
			if err == nil {
				t.Fatal("expected non-nil error, got nil")
			}

			actual := err.Error()
			if !strings.Contains(actual, test.expected) {
				t.Fatalf("spec=%s, expectedError=%s, gotError=%s",
					test.spec, test.expected, actual)
			}
		})
	}
}
