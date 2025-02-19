package parser

import (
	"fmt"
	"strings"
	"time"

	"github.com/gdgvda/cron/internal/matcher"
)

var monthToInt = map[string]uint{
	"jan": 1,
	"feb": 2,
	"mar": 3,
	"apr": 4,
	"may": 5,
	"jun": 6,
	"jul": 7,
	"aug": 8,
	"sep": 9,
	"oct": 10,
	"nov": 11,
	"dec": 12,
}

func ParseMonth(expression string) (matcher.Matcher, error) {
	options, err := splitOptions(expression)
	if err != nil {
		return nil, err
	}
	matches := []matcher.Matcher{}
	for _, option := range options {
		match, err := parseMonth(option)
		if err != nil {
			return nil, err
		}
		matches = append(matches, match)
	}
	return matcher.Or(matches...), nil
}

func parseMonth(expression string) (matcher.Matcher, error) {
	rangeAndStep := strings.Split(expression, "/")
	lowAndHigh := strings.Split(rangeAndStep[0], "-")

	if len(lowAndHigh) > 2 || len(rangeAndStep) > 2 {
		return nil, fmt.Errorf("%s: invalid expression", expression)
	}

	if lowAndHigh[0] == "*" {
		if len(lowAndHigh) > 1 {
			return nil, fmt.Errorf("%s: invalid expression", expression)
		}
		lowAndHigh[0] = "1-12"
	} else {
		if len(lowAndHigh) == 1 && len(rangeAndStep) == 2 {
			lowAndHigh[0] += "-12"
		}
	}

	expression = strings.Join(lowAndHigh, "-")
	if len(rangeAndStep) > 1 {
		expression += "/" + rangeAndStep[1]
	}

	activations, err := span(expression, 1, 12, monthToInt)
	if err != nil {
		return nil, err
	}

	return func(t time.Time) bool {
		for _, month := range activations {
			if t.Month() == time.Month(month) {
				return true
			}
		}
		return false
	}, nil
}
