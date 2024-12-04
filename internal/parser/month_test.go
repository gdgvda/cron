package parser

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestParseMonthMatcher(t *testing.T) {
	tests := []struct {
		spec     string
		time     string
		expected bool
	}{
		{"*", "Feb 2007", true},
		{"*", "Mar 2007", true},

		{"3", "Mar 2025", true},
		{"3", "Mar 2020", true},
		{"3", "Feb 2020", false},
		{"12", "Dec 2021", true},

		{"10-11", "Sep 2023", false},
		{"10-11", "Oct 2023", true},
		{"OCT-11", "Nov 2023", true},
		{"10-11", "Dec 2023", false},
		{"1-12", "Dec 2023", true},

		{"1/3", "Jan 2020", true},
		{"Jan/3", "Apr 2020", true},
		{"jAN/3", "Feb 2020", false},
		{"*/3", "Jan 2020", true},
		{"*/3", "Apr 2020", true},
		{"*/3", "Feb 2020", false},

		{"5/3", "May 2021", true},
		{"5/3", "Aug 2021", true},
		{"5/3", "Nov 2021", true},
		{"5/3", "Jun 2021", false},

		{"Feb-5/3", "Feb 2000", true},
		{"2-5/3", "May 2000", true},
		{"2-5/3", "Aug 2000", false},
		{"2-5/3", "Apr 2000", false},

		{"2-5/3,8", "Aug 2000", true},
		{"2-5/3,7", "Aug 2000", false},

		{"1,2,3", "Jul 2003", false},
		{"1,2,3", "Feb 2003", true},
		{"1,02,3", "Feb 2003", true},
	}

	const layout = "Jan 2006"
	for _, test := range tests {
		t.Run(fmt.Sprintf("spec=%s,now=%s", strings.Replace(test.spec, "/", "|", -1), test.time), func(t *testing.T) {
			time, err := time.Parse(layout, test.time)
			if err != nil {
				t.Fatalf("expected nil error, got %s", err)
			}

			matcher, err := ParseMonth(test.spec)
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

func TestParseMonthErrors(t *testing.T) {
	tests := []struct {
		spec     string
		expected string
	}{
		{"#", "failed to parse"},
		{"?", "failed to parse"},
		{"*-5", "invalid expression"},
		{"*-5/2", "invalid expression"},
		{"5-*", "failed to parse"},
		{"5-*/3", "failed to parse"},
		{"-1", "failed to parse"},
		{"-22", "failed to parse"},
		{"0", "value 0 out of valid range [1, 12]"},
		{"15", "value 15 out of valid range [1, 12]"},
		{"4-13", "value 13 out of valid range [1, 12]"},
		{"4-13/2", "value 13 out of valid range [1, 12]"},
		{"*//2", "invalid expression"},
		{"*/2/", "invalid expression"},
		{"*-2-", "invalid expression"},
		{"2-", "failed to parse"},
		{"1-2-", "invalid expression"},
	}

	for _, test := range tests {
		t.Run(strings.Replace(test.spec, "/", "|", -1), func(t *testing.T) {
			_, err := ParseMonth(test.spec)
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
