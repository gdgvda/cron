package parser

import (
	"fmt"
	"strings"
	"time"

	"github.com/gdgvda/cron/internal/matcher"
)

var domToInt = map[string]uint{}

func ParseDom(expression string) (matcher.Matcher, error) {
	options, err := splitOptions(expression)
	if err != nil {
		return nil, err
	}
	matches := []matcher.Matcher{}
	for _, option := range options {
		match, err := parseDom(option)
		if err != nil {
			return nil, err
		}
		matches = append(matches, match)
	}
	return matcher.Or(matches...), nil
}

func parseDom(expression string) (matcher.Matcher, error) {
	rangeAndStep := strings.Split(expression, "/")
	lowAndHigh := strings.Split(rangeAndStep[0], "-")

	if len(lowAndHigh) > 2 || len(rangeAndStep) > 2 {
		return nil, fmt.Errorf("%s: invalid expression", expression)
	}

	if lowAndHigh[0] == "L" {
		if len(rangeAndStep) > 1 {
			return nil, fmt.Errorf("%s: invalid expression", expression)
		}

		var offset uint = 0
		if len(lowAndHigh) == 2 {
			var err error
			offset, err = mustParseInt(lowAndHigh[1])
			if err != nil {
				return nil, err
			}
			if offset > 30 {
				return nil, fmt.Errorf("%s: invalid amount of days subtracted", expression)
			}
		}
		return func(t time.Time) bool {
			lastDayOfCurrentMonth := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location()).AddDate(0, 1, -1).Day()
			return t.Day() == lastDayOfCurrentMonth-int(offset)
		}, nil
	}

	if strings.HasSuffix(lowAndHigh[0], "W") {
		if len(lowAndHigh) > 1 || len(rangeAndStep) > 1 {
			return nil, fmt.Errorf("%s: invalid expression", expression)
		}
		if lowAndHigh[0] == "LW" {
			return func(t time.Time) bool {
				lastDayOfCurrentMonth := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location()).AddDate(0, 1, -1)
				if lastDayOfCurrentMonth.Weekday() == time.Sunday {
					return t.Day() == lastDayOfCurrentMonth.Day()-2
				}
				if lastDayOfCurrentMonth.Weekday() == time.Saturday {
					return t.Day() == lastDayOfCurrentMonth.Day()-1
				}
				return t.Day() == lastDayOfCurrentMonth.Day()
			}, nil
		}
		dom, err := parseIntOrName(strings.TrimSuffix(lowAndHigh[0], "W"), domToInt)
		if err != nil {
			return nil, err
		}
		if dom < 1 || dom > 31 {
			return nil, fmt.Errorf("%s: value %d out of valid range [1, 31]", expression, dom)
		}
		return func(t time.Time) bool {
			lastDayOfCurrentMonth := uint(time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location()).AddDate(0, 1, -1).Day())
			if dom > lastDayOfCurrentMonth {
				return false
			}
			preferred := time.Date(t.Year(), t.Month(), int(dom), 0, 0, 0, 0, t.Location())
			if preferred.Weekday() == time.Saturday && dom == 1 {
				return t.Day() == 3
			}
			if preferred.Weekday() == time.Saturday && dom != 1 {
				return t.Day() == int(dom)-1
			}
			if preferred.Weekday() == time.Sunday && dom == lastDayOfCurrentMonth {
				return t.Day() == int(dom)-2
			}
			if preferred.Weekday() == time.Sunday && dom != lastDayOfCurrentMonth {
				return t.Day() == int(dom)+1
			}
			return t.Day() == preferred.Day()
		}, nil
	}

	if lowAndHigh[0] == "*" || lowAndHigh[0] == "?" {
		if len(lowAndHigh) > 1 {
			return nil, fmt.Errorf("%s: invalid expression", expression)
		}
		lowAndHigh[0] = "1"
		lowAndHigh = append(lowAndHigh, "31")
	} else {
		if len(lowAndHigh) == 1 && len(rangeAndStep) == 2 {
			lowAndHigh = append(lowAndHigh, "31")
		}
	}

	expression = strings.Join(lowAndHigh, "-")
	if len(rangeAndStep) > 1 {
		expression += "/" + rangeAndStep[1]
	}

	activations, err := span(expression, 1, 31, domToInt)
	if err != nil {
		return nil, err
	}

	return func(t time.Time) bool {
		for _, dom := range activations {
			if uint(t.Day()) == dom {
				return true
			}
		}
		return false
	}, nil
}
