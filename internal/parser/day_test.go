package parser

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestParseDayMatcher(t *testing.T) {
	tests := []struct {
		dom      string
		dow      string
		time     string
		expected bool
	}{
		{"*", "Mon", "Thu Jan 2 2025", false},
		{"*", "Thu", "Thu Jan 2 2025", true},
		{"2", "*", "Fri Jan 3 2025", false},
		{"3", "*", "Fri Jan 3 2025", true},
		{"1", "Thu", "Thu Jan 2 2025", true},
		{"2", "Sat", "Thu Jan 2 2025", true},
		{"*", "1L", "Mon Jan 27 2025", true},
		{"L-4", "4L", "Mon Jan 27 2025", true},
		{"*", "1L", "Mon Jan 20 2025", false},
	}

	const layout = "Mon Jan 2 2006"
	for _, test := range tests {
		t.Run(fmt.Sprintf(
			"dom=%s,dow=%s,now=%s",
			strings.Replace(test.dom, "/", "|", -1),
			strings.Replace(test.dow, "/", "|", -1),
			test.time,
		), func(t *testing.T) {
			time, err := time.Parse(layout, test.time)
			if err != nil {
				t.Fatalf("expected nil error, got %s", err)
			}

			matcher, err := ParseDay(test.dom, test.dow)
			if err != nil {
				t.Fatalf("expected nil error, got %s", err)
			}

			actual := matcher(time)
			if actual != test.expected {
				t.Fatalf("dom=%s, dow=%s, time=%s, expected=%t, got=%t",
					test.dom, test.dow, test.time, test.expected, actual)
			}
		})
	}
}
