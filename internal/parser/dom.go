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

	if lowAndHigh[0] == "*" || lowAndHigh[0] == "?" {
		if len(lowAndHigh) > 1 {
			return nil, fmt.Errorf("%s: invalid expression", expression)
		}
		lowAndHigh[0] = "1-31"
	} else {
		if len(lowAndHigh) == 1 && len(rangeAndStep) == 2 {
			lowAndHigh[0] += "-31"
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
