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
		{"*", "Fri Nov 3 2006", true},
		{"*", "Mon Dec 25 2006", true},
		{"?", "Fri Nov 3 2006", true},
		{"?", "Mon Dec 25 2006", true},

		{"23", "Sat Dec 23 2006", true},
		{"23", "Mon Oct 23 2006", true},
		{"7", "Mon Oct 23 2006", false},
		{"18", "Fri Aug 18 2006", true},

		{"11-12", "Sun Sep 10 2006", false},
		{"11-12", "Mon Sep 11 2006", true},
		{"11-12", "Tue Sep 12 2006", true},
		{"11-12", "Wed Sep 13 2006", false},
		{"1-23", "Fri Feb 17 2006", true},

		{"1/5", "Wed Mar 1 2006", true},
		{"1/5", "Mon Mar 6 2006", true},
		{"1/5", "Tue Mar 7 2006", false},
		{"*/5", "Wed Mar 1 2006", true},
		{"*/5", "Mon Mar 6 2006", true},
		{"*/5", "Tue Mar 7 2006", false},
		{"5/5", "Thu Jan 5 2006", true},
		{"5/5", "Mon Jan 2 2006", false},
		{"5/5", "Tue Jan 10 2006", true},
		{"5/5", "Wed Feb 8 2006", false},

		{"5-17/5", "Wed Apr 5 2006", true},
		{"5-17/5", "Wed May 10 2006", true},
		{"5-17/5", "Sun Aug 20 2006", false},

		{"5-17/5,20", "Sun Aug 20 2006", true},
		{"5-17/5,19", "Sun Aug 20 2006", false},

		{"1,2,3", "Wed Jan 4 2006", false},
		{"1,2,3", "Mon Jan 2 2006", true},
		{"1,02,3", "Mon Jan 2 2006", true},

		{"L", "Thu Jan 2 2025", false},
		{"L", "Fri Jan 31 2025", true},
		{"L", "Wed Feb 7 2024", false},
		{"L", "Wed Feb 28 2024", false},
		{"L", "Fri Feb 28 2025", true},
		{"L", "Thu Feb 29 2024", true},
		{"L", "Fri Nov 29 2024", false},
		{"L", "Sat Nov 30 2024", true},

		{"L-2", "Mon Jan 29 2024", true},
		{"L-2", "Mon Jan 22 2024", false},
		{"L-2,22", "Mon Jan 22 2024", true},
		{"L-30", "Mon Jan 1 2024", true},

		{"17W", "Thu Jul 17 2025", true},
		{"17W", "Fri Jul 18 2025", false},
		{"19W", "Fri Jul 18 2025", true},
		{"19W", "Sat Jul 19 2025", false},
		{"20W", "Mon Jul 21 2025", true},
		{"20W", "Sun Jul 20 2025", false},
		{"1W", "Sat Nov 1 2025", false},
		{"1W", "Sun Nov 2 2025", false},
		{"1W", "Mon Nov 3 2025", true},
		{"30W", "Sun Nov 30 2025", false},
		{"30W", "Sat Nov 29 2025", false},
		{"30W", "Fri Nov 28 2025", true},
		{"31W", "Fri Nov 28 2025", false},
		{"LW", "Fri Nov 28 2025", true},
		{"LW", "Sun Nov 30 2025", false},
		{"LW", "Sat Nov 29 2025", false},
		{"LW", "Fri Feb 28 2025", true},
		{"LW", "Sat May 31 2025", false},
	}

	const layout = "Mon Jan 2 2006"
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
		{"*-W", "invalid expression"},
		{"-2W", "failed to parse"},
		{"34W", "value 34 out of valid range [1, 31]"},
		{"4W/4", "invalid expression"},
		{"4W-4", "invalid expression"},
		{"4W-4/2", "invalid expression"},
		{"W", "failed to parse"},
		{"LW-5", "invalid expression"},
		{"LW/2", "invalid expression"},
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
