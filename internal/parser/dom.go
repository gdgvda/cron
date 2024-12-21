package parser

import (
	"fmt"
	"strings"
	"time"

	"github.com/gdgvda/cron/internal/matcher"
)

var domToInt = map[string]uint{}

func ParseDom(expression string) (matcher.Matcher, error) {
	options := strings.FieldsFunc(expression, func(r rune) bool { return r == ',' })
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
