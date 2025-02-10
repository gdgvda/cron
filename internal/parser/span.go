package parser

import (
	"fmt"
	"strconv"
	"strings"
)

func span(expression string, min, max uint, strToInt map[string]uint) ([]uint, error) {
	rangeAndStep := strings.Split(expression, "/")
	lowAndHigh := strings.Split(rangeAndStep[0], "-")

	low, err := parseIntOrName(lowAndHigh[0], strToInt)
	if err != nil {
		return nil, err
	}
	if low < min || low > max {
		return nil, fmt.Errorf("%s: value %d out of valid range [%d, %d]", expression, low, min, max)
	}

	var high uint
	switch len(lowAndHigh) {
	case 1:
		high = low
	case 2:
		high, err = parseIntOrName(lowAndHigh[1], strToInt)
		if err != nil {
			return nil, err
		}
		if high < min || high > max {
			return nil, fmt.Errorf("%s: value %d out of valid range [%d, %d]", expression, high, min, max)
		}
		if high < low {
			return nil, fmt.Errorf("%s: beginning of range (%d) beyond end of range (%d)", expression, low, high)
		}
	default:
		return nil, fmt.Errorf("too many hyphens: %s", expression)
	}

	var step uint
	switch len(rangeAndStep) {
	case 1:
		step = 1
	case 2:
		step, err = mustParseInt(rangeAndStep[1])
		if err != nil {
			return nil, err
		}

		// Special handling: "N/step" means "N-max/step".
		if len(lowAndHigh) == 1 {
			high = max
		}
	default:
		return nil, fmt.Errorf("too many slashes: %s", expression)
	}

	if step <= 0 {
		return nil, fmt.Errorf("step should be > 0, got %d", step)
	}

	result := []uint{}
	for i := low; i <= high; i += step {
		result = append(result, i)
	}
	return result, nil
}

func parseIntOrName(expr string, names map[string]uint) (uint, error) {
	if names != nil {
		if namedInt, ok := names[strings.ToLower(expr)]; ok {
			return namedInt, nil
		}
	}
	return mustParseInt(expr)
}

func mustParseInt(expr string) (uint, error) {
	num, err := strconv.Atoi(expr)
	if err != nil {
		return 0, fmt.Errorf("failed to parse int from %s: %s", expr, err)
	}
	if num < 0 {
		return 0, fmt.Errorf("negative number (%d) not allowed: %s", num, expr)
	}

	return uint(num), nil
}
