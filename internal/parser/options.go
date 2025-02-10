package parser

import (
	"fmt"
	"strings"
)

func splitOptions(expression string) ([]string, error) {
	options := strings.FieldsFunc(expression, func(r rune) bool { return r == ',' })
	if len(options) == 0 {
		return nil, fmt.Errorf("invalid expression: empty list")
	}
	return options, nil
}
