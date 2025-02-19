package parser

import (
	"fmt"
	"strings"
	"time"

	"github.com/gdgvda/cron/internal/matcher"
)

var minuteToInt = map[string]uint{}

func ParseMinute(expression string) (matcher.Matcher, error) {
	options, err := splitOptions(expression)
	if err != nil {
		return nil, err
	}
	matches := []matcher.Matcher{}
	for _, option := range options {
		match, err := parseMinute(option)
		if err != nil {
			return nil, err
		}
		matches = append(matches, match)
	}
	return matcher.Or(matches...), nil
}

func parseMinute(expression string) (matcher.Matcher, error) {
	rangeAndStep := strings.Split(expression, "/")
	lowAndHigh := strings.Split(rangeAndStep[0], "-")

	if len(lowAndHigh) > 2 || len(rangeAndStep) > 2 {
		return nil, fmt.Errorf("%s: invalid expression", expression)
	}

	if lowAndHigh[0] == "*" {
		if len(lowAndHigh) > 1 {
			return nil, fmt.Errorf("%s: invalid expression", expression)
		}
		lowAndHigh[0] = "0-59"
	} else {
		if len(lowAndHigh) == 1 && len(rangeAndStep) == 2 {
			lowAndHigh[0] += "-59"
		}
	}

	expression = strings.Join(lowAndHigh, "-")
	if len(rangeAndStep) > 1 {
		expression += "/" + rangeAndStep[1]
	}

	activations, err := span(expression, 0, 59, minuteToInt)
	if err != nil {
		return nil, err
	}

	return func(t time.Time) bool {
		for _, minute := range activations {
			if uint(t.Minute()) == minute {
				return true
			}
		}
		return false
	}, nil
}
