package matcher

import "time"

type Matcher func(time.Time) bool

func Or(matchers ...Matcher) Matcher {
	return func(t time.Time) bool {
		for _, m := range matchers {
			if m(t) {
				return true
			}
		}
		return false
	}
}

func And(matchers ...Matcher) Matcher {
	return func(t time.Time) bool {
		for _, m := range matchers {
			if !m(t) {
				return false
			}
		}
		return true
	}
}
