package cron

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

var secondParser = NewParser(Second | Minute | Hour | Dom | Month | DowOptional | Descriptor)

func TestParseScheduleErrors(t *testing.T) {
	var tests = []struct{ expr, err string }{
		{"* 5 j * * *", "failed to parse int from"},
		{"@every Xm", "failed to parse duration"},
		{"@every 1ns", "delay must be at least one second"},
		{"@every 1h1ms", "delay must be a multiple of one second"},
		{"@unrecognized", "unrecognized descriptor"},
		{"* * * *", "expected 5 to 6 fields"},
		{"", "empty spec string"},
	}
	for _, c := range tests {
		actual, err := secondParser.Parse(c.expr)
		if err == nil || !strings.Contains(err.Error(), c.err) {
			t.Errorf("%s => expected %v, got %v", c.expr, c.err, err)
		}
		if actual != nil {
			t.Errorf("expected nil schedule on error, got %v", actual)
		}
	}
}

func TestParseSchedule(t *testing.T) {
	optionalSecondParser := NewParser(SecondOptional | Minute | Hour | Dom | Month | Dow | Descriptor)
	layout := time.RFC3339
	entries := []struct {
		now      string
		parser   Parser
		expr     string
		expected string
	}{
		{"2025-01-01T18:00:00Z", secondParser, "0 5 * * * *", "2025-01-01T18:05:00Z"},
		{"2025-01-01T18:06:00Z", secondParser, "0 5 * * * *", "2025-01-01T19:05:00Z"},
		{"2025-01-01T18:00:00Z", standardParser, "5 * * * *", "2025-01-01T18:05:00Z"},
		{"2025-01-01T18:00:00Z", secondParser, "CRON_TZ=UTC  0 5 * * * *", "2025-01-01T18:05:00Z"},
		{"2025-01-01T18:00:00Z", standardParser, "CRON_TZ=UTC  5 * * * *", "2025-01-01T18:05:00Z"},
		{"2025-01-01T18:00:00Z", secondParser, "CRON_TZ=Asia/Tokyo 0 5 * * * *", "2025-01-02T03:05:00+09:00"},
		{"2025-01-01T18:00:00Z", secondParser, "@every 5m", "2025-01-01T18:05:00Z"},
		{"2025-01-01T18:00:00Z", secondParser, "@midnight", "2025-01-02T00:00:00Z"},
		{"2025-01-01T18:00:00Z", secondParser, "TZ=UTC  @midnight", "2025-01-02T00:00:00Z"},
		{"2025-01-01T18:00:00Z", secondParser, "TZ=Asia/Tokyo @midnight", "2025-01-03T00:00:00+09:00"},
		{"2025-01-01T18:00:00Z", secondParser, "@yearly", "2026-01-01T00:00:00Z"},
		{"2025-01-01T18:00:00Z", secondParser, "@annually", "2026-01-01T00:00:00Z"},
		{"2025-01-01T18:00:00Z", secondParser, "* 5 * * * *", "2025-01-01T18:05:00Z"},
		{"2025-01-01T18:05:00Z", secondParser, "* 5 * * * *", "2025-01-01T18:05:01Z"},
		{"2025-01-01T18:00:00Z", optionalSecondParser, "0 5 * * * *", "2025-01-01T18:05:00Z"},
		{"2025-01-01T18:00:00Z", optionalSecondParser, "5 5 * * * *", "2025-01-01T18:05:05Z"},
		{"2025-01-01T18:00:00Z", optionalSecondParser, "5 * * * *", "2025-01-01T18:05:00Z"},
	}

	for _, c := range entries {
		t.Run(strings.Replace(c.expr, "/", "|", -1), func(t *testing.T) {
			schedule, err := c.parser.Parse(c.expr)
			if err != nil {
				t.Fatalf("%s => unexpected error %v", c.expr, err)
			}
			now, err := time.Parse(layout, c.now)
			if err != nil {
				t.Fatalf("%s => unexpected error %v", c.now, err)
			}
			actual := schedule.Next(now).In(time.UTC)
			expected, err := time.Parse(layout, c.expected)
			if err != nil {
				t.Fatalf("%s => unexpected error %v", c.expected, err)
			}
			expected = expected.In(time.UTC)
			if actual != expected {
				t.Fatalf("%s => expected %s, got %s", c.expr, expected, actual)
			}
		})
	}
}

func TestNormalizeFields(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		options  ParseOption
		expected []string
	}{
		{
			"AllFields_NoOptional",
			[]string{"0", "5", "*", "*", "*", "*"},
			Second | Minute | Hour | Dom | Month | Dow | Descriptor,
			[]string{"0", "5", "*", "*", "*", "*"},
		},
		{
			"AllFields_SecondOptional_Provided",
			[]string{"0", "5", "*", "*", "*", "*"},
			SecondOptional | Minute | Hour | Dom | Month | Dow | Descriptor,
			[]string{"0", "5", "*", "*", "*", "*"},
		},
		{
			"AllFields_SecondOptional_NotProvided",
			[]string{"5", "*", "*", "*", "*"},
			SecondOptional | Minute | Hour | Dom | Month | Dow | Descriptor,
			[]string{"0", "5", "*", "*", "*", "*"},
		},
		{
			"SubsetFields_NoOptional",
			[]string{"5", "15", "*"},
			Hour | Dom | Month,
			[]string{"0", "0", "5", "15", "*", "*"},
		},
		{
			"SubsetFields_DowOptional_Provided",
			[]string{"5", "15", "*", "4"},
			Hour | Dom | Month | DowOptional,
			[]string{"0", "0", "5", "15", "*", "4"},
		},
		{
			"SubsetFields_DowOptional_NotProvided",
			[]string{"5", "15", "*"},
			Hour | Dom | Month | DowOptional,
			[]string{"0", "0", "5", "15", "*", "*"},
		},
		{
			"SubsetFields_SecondOptional_NotProvided",
			[]string{"5", "15", "*"},
			SecondOptional | Hour | Dom | Month,
			[]string{"0", "0", "5", "15", "*", "*"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual, err := normalizeFields(test.input, test.options)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(actual, test.expected) {
				t.Errorf("expected %v, got %v", test.expected, actual)
			}
		})
	}
}

func TestNormalizeFields_Errors(t *testing.T) {
	tests := []struct {
		name    string
		input   []string
		options ParseOption
		err     string
	}{
		{
			"TwoOptionals",
			[]string{"0", "5", "*", "*", "*", "*"},
			SecondOptional | Minute | Hour | Dom | Month | DowOptional,
			"",
		},
		{
			"TooManyFields",
			[]string{"0", "5", "*", "*"},
			SecondOptional | Minute | Hour,
			"",
		},
		{
			"NoFields",
			[]string{},
			SecondOptional | Minute | Hour,
			"",
		},
		{
			"TooFewFields",
			[]string{"*"},
			SecondOptional | Minute | Hour,
			"",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual, err := normalizeFields(test.input, test.options)
			if err == nil {
				t.Errorf("expected an error, got none. results: %v", actual)
			}
			if !strings.Contains(err.Error(), test.err) {
				t.Errorf("expected error %q, got %q", test.err, err.Error())
			}
		})
	}
}

func TestStandardSpecSchedule(t *testing.T) {
	layout := time.RFC3339
	entries := []struct {
		now      string
		expr     string
		expected string
		err      string
	}{
		{"2025-01-01T18:00:00Z", "5 * * * *", "2025-01-01T18:05:00Z", ""},
		{"2025-01-01T18:02:00Z", "@every 5m", "2025-01-01T18:07:00Z", ""},
		{"", "5 j * * *", "", "failed to parse int from"},
		{"", "* * * *", "", "expected exactly 5 fields"},
	}

	for _, c := range entries {
		t.Run(strings.Replace(c.expr, "/", "|", -1), func(t *testing.T) {
			schedule, err := ParseStandard(c.expr)
			if len(c.err) != 0 && (err == nil || !strings.Contains(err.Error(), c.err)) {
				t.Fatalf("%s => expected %v, got %v", c.expr, c.err, err)
			}
			if len(c.err) == 0 && err != nil {
				t.Fatalf("%s => unexpected error %v", c.expr, err)
			}

			if err != nil {
				return
			}
			now, err := time.Parse(layout, c.now)
			if err != nil {
				t.Fatalf("%s => unexpected error %v", c.now, err)
			}
			actual := schedule.Next(now).In(time.UTC)
			expected, err := time.Parse(layout, c.expected)
			if err != nil {
				t.Fatalf("%s => unexpected error %v", c.expected, err)
			}
			expected = expected.In(time.UTC)
			if actual != expected {
				t.Fatalf("%s => expected %s, got %s", c.expr, expected, actual)
			}
		})
	}
}

func TestNoDescriptorParser(t *testing.T) {
	parser := NewParser(Minute | Hour)
	_, err := parser.Parse("@every 1m")
	if err == nil {
		t.Error("expected an error, got none")
	}
}
