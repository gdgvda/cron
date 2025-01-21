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
		{"*", "Nov 3 2006", true},
		{"*", "Dec 25 2006", true},
		{"?", "Nov 3 2006", true},
		{"?", "Dec 25 2006", true},

		{"23", "Dec 23 2006", true},
		{"23", "Oct 23 2006", true},
		{"7", "Oct 23 2006", false},
		{"18", "Aug 18 2006", true},

		{"11-12", "Sep 10 2006", false},
		{"11-12", "Sep 11 2006", true},
		{"11-12", "Sep 12 2006", true},
		{"11-12", "Sep 13 2006", false},
		{"1-23", "Feb 17 2006", true},

		{"1/5", "Mar 1 2006", true},
		{"1/5", "Mar 6 2006", true},
		{"1/5", "Mar 7 2006", false},
		{"*/5", "Mar 1 2006", true},
		{"*/5", "Mar 6 2006", true},
		{"*/5", "Mar 7 2006", false},
		{"5/5", "Jan 5 2006", true},
		{"5/5", "Jan 2 2006", false},
		{"5/5", "Jan 10 2006", true},
		{"5/5", "Feb 8 2006", false},

		{"5-17/5", "Apr 5 2006", true},
		{"5-17/5", "May 10 2006", true},
		{"5-17/5", "Aug 20 2006", false},

		{"5-17/5,20", "Aug 20 2006", true},
		{"5-17/5,19", "Aug 20 2006", false},

		{"1,2,3", "Jan 4 2006", false},
		{"1,2,3", "Jan 2 2006", true},
		{"1,02,3", "Jan 2 2006", true},

		{"L", "Jan 2 2025", false},
		{"L", "Jan 31 2025", true},
		{"L", "Feb 7 2024", false},
		{"L", "Feb 28 2024", false},
		{"L", "Feb 28 2025", true},
		{"L", "Feb 29 2024", true},
		{"L", "Nov 29 2024", false},
		{"L", "Nov 30 2024", true},

		{"L-2", "Jan 29 2024", true},
		{"L-2", "Jan 22 2024", false},
		{"L-2,22", "Jan 22 2024", true},
		{"L-30", "Jan 1 2024", true},
	}

	const layout = "Jan 2 2006"
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
		{"L/4", "invalid expression"},
		{"L-3/4", "invalid expression"},
		{"L-*", "failed to parse"},
		{"*-L", "invalid expression"},
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
