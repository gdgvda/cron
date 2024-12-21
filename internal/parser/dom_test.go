package parser

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestParseDomMatcher(t *testing.T) {
	tests := []struct {
		spec     string
		time     string
		expected bool
	}{
		{"*", "Nov 3", true},
		{"*", "Dec 25", true},
		{"?", "Nov 3", true},
		{"?", "Dec 25", true},

		{"23", "Dec 23", true},
		{"23", "Oct 23", true},
		{"7", "Oct 23", false},
		{"18", "Aug 18", true},

		{"11-12", "Sep 10", false},
		{"11-12", "Sep 11", true},
		{"11-12", "Sep 12", true},
		{"11-12", "Sep 13", false},
		{"1-23", "Feb 17", true},

		{"1/5", "Mar 1", true},
		{"1/5", "Mar 6", true},
		{"1/5", "Mar 7", false},
		{"*/5", "Mar 1", true},
		{"*/5", "Mar 6", true},
		{"*/5", "Mar 7", false},
		{"5/5", "Jan 5", true},
		{"5/5", "Jan 2", false},
		{"5/5", "Jan 10", true},
		{"5/5", "Feb 8", false},

		{"5-17/5", "Apr 5", true},
		{"5-17/5", "May 10", true},
		{"5-17/5", "Aug 20", false},

		{"5-17/5,20", "Aug 20", true},
		{"5-17/5,19", "Aug 20", false},

		{"1,2,3", "Jan 4", false},
		{"1,2,3", "Jan 2", true},
		{"1,02,3", "Jan 2", true},
	}

	const layout = "Jan 2"
	for _, test := range tests {
		t.Run(fmt.Sprintf("spec=%s,now=%s", strings.Replace(test.spec, "/", "|", -1), test.time), func(t *testing.T) {
			time, err := time.Parse(layout, test.time)
			if err != nil {
				t.Fatalf("expected nil error, got %s", err)
			}

			matcher, err := ParseDom(test.spec)
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

func TestParseDomErrors(t *testing.T) {
	tests := []struct {
		spec     string
		expected string
	}{
		{"#", "failed to parse"},
		{"*-5", "invalid expression"},
		{"*-5/2", "invalid expression"},
		{"5-*", "failed to parse"},
		{"5-*/22", "failed to parse"},
		{"-1", "failed to parse"},
		{"-22", "failed to parse"},
		{"32", "value 32 out of valid range [1, 31]"},
		{"22-32", "value 32 out of valid range [1, 31]"},
		{"22-33/36", "value 33 out of valid range [1, 31]"},
		{"22-23/ABC", "failed to parse"},
		{"*//2", "invalid expression"},
		{"*/2/", "invalid expression"},
		{"*-2-", "invalid expression"},
		{"2-", "failed to parse"},
		{"1-2-", "invalid expression"},
	}

	for _, test := range tests {
		t.Run(strings.Replace(test.spec, "/", "|", -1), func(t *testing.T) {
			_, err := ParseDom(test.spec)
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
