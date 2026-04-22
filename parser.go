package cron

import (
	"fmt"
	"strings"
	"time"

	"github.com/gdgvda/cron/internal/parser"
)

// Configuration options for creating a parser. Most options specify which
// fields should be included, while others enable features. If a field is not
// included the parser will assume a default value. These options do not change
// the order fields are parse in.
type ParseOption int

const (
	Second         ParseOption = 1 << iota // Seconds field, default 0
	SecondOptional                         // Optional seconds field, default 0
	Minute                                 // Minutes field, default 0
	Hour                                   // Hours field, default 0
	Dom                                    // Day of month field, default *
	Month                                  // Month field, default *
	Dow                                    // Day of week field, default *
	DowOptional                            // Optional day of week field, default *
	Descriptor                             // Allow descriptors such as @monthly, @weekly, etc.
)

var places = []ParseOption{
	Second,
	Minute,
	Hour,
	Dom,
	Month,
	Dow,
}

var defaults = []string{
	"0",
	"0",
	"0",
	"*",
	"*",
	"*",
}

// A default DefaultParser that can be configured.
type DefaultParser struct {
	options ParseOption
}

// NewDefaultParser creates a DefaultParser with custom options.
//
// It panics if more than one Optional is given, since it would be impossible to
// correctly infer which optional is provided or missing in general.
//
// Examples
//
//	// Standard parser without descriptors
//	specParser, _ := NewDefaultParser(Minute | Hour | Dom | Month | Dow)
//	sched, err := specParser.Parse("0 0 15 */3 *")
//
//	// Same as above, just excludes time fields
//	specParser, _ := NewDefaultParser(Dom | Month | Dow)
//	sched, err := specParser.Parse("15 */3 *")
//
//	// Same as above, just makes Dow optional
//	specParser, _ := NewDefaultParser(Dom | Month | DowOptional)
//	sched, err := specParser.Parse("15 */3")
func NewDefaultParser(options ParseOption) (*DefaultParser, error) {
	optionals := 0
	if options&DowOptional > 0 {
		optionals++
	}
	if options&SecondOptional > 0 {
		optionals++
	}
	if optionals > 1 {
		return nil, fmt.Errorf("multiple optionals may not be configured")
	}
	return &DefaultParser{options}, nil
}

// Parse returns a new crontab schedule representing the given spec.
// It returns a descriptive error if the spec is not valid.
// It accepts crontab specs and features configured by NewDefaultParser.
func (p *DefaultParser) Parse(spec string) (*DefaultSchedule, error) {
	if len(spec) == 0 {
		return nil, fmt.Errorf("empty spec string")
	}

	// Extract timezone if present
	var loc = time.Local
	if strings.HasPrefix(spec, "TZ=") || strings.HasPrefix(spec, "CRON_TZ=") {
		var err error
		i := strings.Index(spec, " ")
		if i == -1 {
			return nil, fmt.Errorf("invalid location descriptior: %s", spec)
		}
		eq := strings.Index(spec, "=")
		if loc, err = time.LoadLocation(spec[eq+1 : i]); err != nil {
			return nil, fmt.Errorf("provided bad location %s: %v", spec[eq+1:i], err)
		}
		spec = strings.TrimSpace(spec[i:])
	}

	// Handle named schedules (descriptors), if configured
	if strings.HasPrefix(spec, "@") {
		if p.options&Descriptor == 0 {
			return nil, fmt.Errorf("parser does not accept descriptors: %v", spec)
		}
		return parseDescriptor(spec, loc)
	}

	// Split on whitespace.
	fields := strings.Fields(spec)

	// Validate & fill in any omitted or optional fields
	var err error
	fields, err = normalizeFields(fields, p.options)
	if err != nil {
		return nil, err
	}

	second, err := parser.ParseSecond(fields[0])
	if err != nil {
		return nil, err
	}
	minute, err := parser.ParseMinute(fields[1])
	if err != nil {
		return nil, err
	}
	hour, err := parser.ParseHour(fields[2])
	if err != nil {
		return nil, err
	}
	day, err := parser.ParseDay(fields[3], fields[5])
	if err != nil {
		return nil, err
	}
	month, err := parser.ParseMonth(fields[4])
	if err != nil {
		return nil, err
	}

	return &DefaultSchedule{
		secondMatch: second,
		minuteMatch: minute,
		hourMatch:   hour,
		dayMatch:    day,
		monthMatch:  month,
		location:    loc,
	}, nil
}

// normalizeFields takes a subset set of the time fields and returns the full set
// with defaults (zeroes) populated for unset fields.
//
// As part of performing this function, it also validates that the provided
// fields are compatible with the configured options.
func normalizeFields(fields []string, options ParseOption) ([]string, error) {
	// Validate optionals & add their field to options
	optionals := 0
	if options&SecondOptional > 0 {
		options |= Second
		optionals++
	}
	if options&DowOptional > 0 {
		options |= Dow
		optionals++
	}
	if optionals > 1 {
		return nil, fmt.Errorf("multiple optionals may not be configured")
	}

	// Figure out how many fields we need
	max := 0
	for _, place := range places {
		if options&place > 0 {
			max++
		}
	}
	min := max - optionals

	// Validate number of fields
	if count := len(fields); count < min || count > max {
		if min == max {
			return nil, fmt.Errorf("expected exactly %d fields, found %d: %s", min, count, fields)
		}
		return nil, fmt.Errorf("expected %d to %d fields, found %d: %s", min, max, count, fields)
	}

	// Populate the optional field if not provided
	if min < max && len(fields) == min {
		switch {
		case options&DowOptional > 0:
			fields = append(fields, defaults[5]) // TODO: improve access to default
		case options&SecondOptional > 0:
			fields = append([]string{defaults[0]}, fields...)
		default:
			return nil, fmt.Errorf("unknown optional field")
		}
	}

	// Populate all fields not part of options with their defaults
	n := 0
	expandedFields := make([]string, len(places))
	copy(expandedFields, defaults)
	for i, place := range places {
		if options&place > 0 {
			expandedFields[i] = fields[n]
			n++
		}
	}
	return expandedFields, nil
}

var standardParser, _ = NewDefaultParser(
	Minute | Hour | Dom | Month | Dow | Descriptor,
)

// ParseStandard returns a new crontab schedule representing the given
// standardSpec (https://en.wikipedia.org/wiki/Cron). It requires 5 entries
// representing: minute, hour, day of month, month and day of week, in that
// order. It returns a descriptive error if the spec is not valid.
//
// It accepts
//   - Standard crontab specs, e.g. "* * * * ?"
//   - Descriptors, e.g. "@midnight", "@every 1h30m"
func ParseStandard(standardSpec string) (Schedule, error) {
	return standardParser.Parse(standardSpec)
}

// parseDescriptor returns a predefined schedule for the expression, or error if none matches.
func parseDescriptor(descriptor string, loc *time.Location) (*DefaultSchedule, error) {
	switch descriptor {
	case "@yearly", "@annually":
		return create("0", "0", "0", "1", "1", "*", loc)

	case "@monthly":
		return create("0", "0", "0", "1", "*", "*", loc)

	case "@weekly":
		return create("0", "0", "0", "*", "*", "0", loc)

	case "@daily", "@midnight":
		return create("0", "0", "0", "*", "*", "*", loc)

	case "@hourly":
		return create("0", "0", "*", "*", "*", "*", loc)
	}

	const everyPrefix = "@every "
	if strings.HasPrefix(descriptor, everyPrefix) {
		duration, err := time.ParseDuration(descriptor[len(everyPrefix):])
		if err != nil {
			return nil, fmt.Errorf("failed to parse duration %s: %s", descriptor, err)
		}
		return every(duration)
	}

	return nil, fmt.Errorf("unrecognized descriptor: %s", descriptor)
}

func create(second, minute, hour, dom, month, dow string, location *time.Location) (*DefaultSchedule, error) {
	secondMatch, err := parser.ParseSecond(second)
	if err != nil {
		return nil, err
	}
	minuteMatch, err := parser.ParseMinute(minute)
	if err != nil {
		return nil, err
	}
	hourMatch, err := parser.ParseHour(hour)
	if err != nil {
		return nil, err
	}
	monthMatch, err := parser.ParseMonth(month)
	if err != nil {
		return nil, err
	}
	dayMatch, err := parser.ParseDay(dom, dow)
	if err != nil {
		return nil, err
	}
	return &DefaultSchedule{
		secondMatch: secondMatch,
		minuteMatch: minuteMatch,
		hourMatch:   hourMatch,
		dayMatch:    dayMatch,
		monthMatch:  monthMatch,
		location:    location,
	}, nil
}

// every returns a crontab Schedule that activates once every duration.
// Delays that are less than on second or not a multiple of a second will return an error.
func every(duration time.Duration) (*DefaultSchedule, error) {
	if duration < time.Second {
		return nil, fmt.Errorf("delay must be at least one second but was %s", duration.String())
	} else if duration%time.Second != 0 {
		return nil, fmt.Errorf("delay must be a multiple of one second but was %s", duration.String())
	}
	return &DefaultSchedule{delay: duration}, nil
}
