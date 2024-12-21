package parser

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestParseMinuteMatcher(t *testing.T) {
	tests := []struct {
		spec     string
		time     string
		expected bool
	}{
		{"*", "15:21:00", true},
		{"*", "23:59:00", true},

		{"23", "15:23:00", true},
		{"23", "15:23:17", true},
		{"23", "15:24:00", false},
		{"59", "23:59:00", true},

		{"11-12", "15:10:00", false},
		{"11-12", "15:11:00", true},
		{"11-12", "15:12:00", true},
		{"11-12", "15:13:00", false},
		{"0-59", "15:23:17", true},

		{"0/15", "15:00:00", true},
		{"0/15", "15:45:00", true},
		{"0/15", "15:40:00", false},
		{"*/15", "15:00:00", true},
		{"*/15", "15:45:00", true},
		{"*/15", "15:40:00", false},

		{"5/15", "15:05:00", true},
		{"5/15", "15:20:00", true},
		{"5/15", "15:50:00", true},
		{"5/15", "15:55:00", false},

		{"5-22/15", "15:05:00", true},
		{"5-22/15", "15:20:00", true},
		{"5-22/15", "15:50:00", false},
		{"5-22/15", "15:55:00", false},

		{"5-22/15,50", "15:50:00", true},
		{"5-22/15,49", "15:50:00", false},

		{"1,2,3", "15:00:00", false},
		{"1,2,3", "15:02:16", true},
	}

	const layout = "15:04:05"
	for _, test := range tests {
		t.Run(fmt.Sprintf("spec=%s,now=%s", strings.Replace(test.spec, "/", "|", -1), test.time), func(t *testing.T) {
			time, err := time.Parse(layout, test.time)
			if err != nil {
				t.Fatalf("expected nil error, got %s", err)
			}

			matcher, err := ParseMinute(test.spec)
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

func TestParseMinuteErrors(t *testing.T) {
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
		{"60", "value 60 out of valid range [0, 59]"},
		{"22-60", "value 60 out of valid range [0, 59]"},
		{"22-60/33", "value 60 out of valid range [0, 59]"},
		{"*//2", "invalid expression"},
		{"*/2/", "invalid expression"},
		{"*-2-", "invalid expression"},
		{"2-", "failed to parse"},
		{"1-2-", "invalid expression"},
	}

	for _, test := range tests {
		t.Run(strings.Replace(test.spec, "/", "|", -1), func(t *testing.T) {
			_, err := ParseMinute(test.spec)
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
