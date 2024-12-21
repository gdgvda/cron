package parser

import (
	"strings"

	"github.com/gdgvda/cron/internal/matcher"
)

func ParseDay(dom, dow string) (matcher.Matcher, error) {
	domMatcher, err := ParseDom(dom)
	if err != nil {
		return nil, err
	}
	dowMatcher, err := ParseDow(dow)
	if err != nil {
		return nil, err
	}

	doms := strings.FieldsFunc(dom, func(r rune) bool { return r == ',' })
	for _, v := range doms {
		if v == "*" || v == "?" {
			return matcher.And(domMatcher, dowMatcher), nil
		}
	}

	dows := strings.FieldsFunc(dow, func(r rune) bool { return r == ',' })
	for _, v := range dows {
		if v == "*" || v == "?" {
			return matcher.And(domMatcher, dowMatcher), nil
		}
	}

	return matcher.Or(domMatcher, dowMatcher), nil
}
