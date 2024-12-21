package parser

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestParseSecondMatcher(t *testing.T) {
	tests := []struct {
		spec     string
		time     string
		expected bool
	}{
		{"*", "15:00:21", true},
		{"*", "23:00:59", true},

		{"23", "15:00:23", true},
		{"23", "15:17:23", true},
		{"23", "15:00:24", false},
		{"59", "23:00:59", true},

		{"11-12", "15:00:10", false},
		{"11-12", "15:00:11", true},
		{"11-12", "15:00:12", true},
		{"11-12", "15:00:13", false},
		{"0-59", "15:17:23", true},

		{"0/15", "15:00:00", true},
		{"0/15", "15:00:45", true},
		{"0/15", "15:00:40", false},
		{"*/15", "15:00:00", true},
		{"*/15", "15:00:45", true},
		{"*/15", "15:00:40", false},

		{"5/15", "15:00:05", true},
		{"5/15", "15:00:20", true},
		{"5/15", "15:00:50", true},
		{"5/15", "15:00:55", false},

		{"5-22/15", "15:00:05", true},
		{"5-22/15", "15:00:20", true},
		{"5-22/15", "15:00:50", false},
		{"5-22/15", "15:00:55", false},

		{"5-22/15,50", "15:00:50", true},
		{"5-22/15,49", "15:00:50", false},

		{"1,2,3", "15:00:00", false},
		{"1,2,3", "15:16:02", true},
		{"1,02,3", "15:16:02", true},
	}

	const layout = "15:04:05"
	for _, test := range tests {
		t.Run(fmt.Sprintf("spec=%s,now=%s", strings.Replace(test.spec, "/", "|", -1), test.time), func(t *testing.T) {
			time, err := time.Parse(layout, test.time)
			if err != nil {
				t.Fatalf("expected nil error, got %s", err)
			}

			matcher, err := ParseSecond(test.spec)
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

func TestParseSecondErrors(t *testing.T) {
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
			_, err := ParseSecond(test.spec)
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
