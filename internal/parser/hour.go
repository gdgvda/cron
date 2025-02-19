package parser

import (
	"fmt"
	"strings"
	"time"

	"github.com/gdgvda/cron/internal/matcher"
)

var hourToInt = map[string]uint{}

func ParseHour(expression string) (matcher.Matcher, error) {
	options, err := splitOptions(expression)
	if err != nil {
		return nil, err
	}
	matches := []matcher.Matcher{}
	for _, option := range options {
		match, err := parseHour(option)
		if err != nil {
			return nil, err
		}
		matches = append(matches, match)
	}
	return matcher.Or(matches...), nil
}

func parseHour(expression string) (matcher.Matcher, error) {
	rangeAndStep := strings.Split(expression, "/")
	lowAndHigh := strings.Split(rangeAndStep[0], "-")

	if len(lowAndHigh) > 2 || len(rangeAndStep) > 2 {
		return nil, fmt.Errorf("%s: invalid expression", expression)
	}

	if lowAndHigh[0] == "*" {
		if len(lowAndHigh) > 1 {
			return nil, fmt.Errorf("%s: invalid expression", expression)
		}
		lowAndHigh[0] = "0-23"
	} else {
		if len(lowAndHigh) == 1 && len(rangeAndStep) == 2 {
			lowAndHigh[0] += "-23"
		}
	}

	expression = strings.Join(lowAndHigh, "-")
	if len(rangeAndStep) > 1 {
		expression += "/" + rangeAndStep[1]
	}

	activations, err := span(expression, 0, 23, hourToInt)
	if err != nil {
		return nil, err
	}

	return func(t time.Time) bool {
		for _, hour := range activations {
			if uint(t.Hour()) == hour {
				return true
			}
		}
		return false
	}, nil
}
