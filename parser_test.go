package cron

import (
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode"
)

var secondParser = NewParser(Second | Minute | Hour | Dom | Month | DowOptional | Descriptor)
var optionalSecondParser = NewParser(SecondOptional | Minute | Hour | Dom | Month | Dow | Descriptor)

func TestParseScheduleErrors(t *testing.T) {
	var tests = []struct{ expr, err string }{
		{"* 5 j * * *", "failed to parse int from"},
		{"@every Xm", "failed to parse duration"},
		{"@every 1ns", "delay must be at least one second"},
		{"@every 1h1ms", "delay must be a multiple of one second"},
		{"@unrecognized", "unrecognized descriptor"},
		{"* * * *", "expected 5 to 6 fields"},
		{"", "empty spec string"},
		{"* * * * L", "failed to parse"},
		{"* * * L/2 *", "L/2: invalid expression"},
		{"* * * L-4/2 *", "L-4/2: invalid expression"},
		{"* * * L-31 *", "L-31: invalid amount of days subtracted"},
	}
	for _, c := range tests {
		t.Run(strings.Replace(c.expr, "/", "|", -1), func(t *testing.T) {
			actual, err := secondParser.Parse(c.expr)
			if err == nil || !strings.Contains(err.Error(), c.err) {
				t.Errorf("%s => expected %v, got %v", c.expr, c.err, err)
			}
			if actual != nil {
				t.Errorf("expected nil schedule on error, got %v", actual)
			}
		})
	}
}

func TestParseSchedule(t *testing.T) {
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
		{"", "TZ=", "", "invalid location descriptior: TZ="},
		{"", "CRON_TZ=0", "", "invalid location descriptior: CRON_TZ=0"},
		{"", ", 0 1 1 0", "", "invalid expression: empty list"},
		{"", "0 , 1 1 0", "", "invalid expression: empty list"},
		{"", "0 1 , 1 0", "", "invalid expression: empty list"},
		{"", "0 2 1 , 0", "", "invalid expression: empty list"},
		{"", "0 3 1 1 ,", "", "invalid expression: empty list"},
		{"", "0 0 * 1 1-0", "", "beginning of range (1) beyond end of range (0)"},
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

// can be started like `go test -fuzz=FuzzParser` and will run until a failure is found or manually stopped
func FuzzParser(f *testing.F) {
	testcases := []string{
		"* * * * *",
		"* * * * * *",
		"5 * * * *",
		"0 22 * * 1-5",
		"23 0-20/2 * * *",
		"5 4 * * sun",
		"0 0,12 1 */2 *",
		"* * L * *",
		"* * L-28 2 *",
		"* * 29 FEB *",
		"* * 29 jan,FEB,mar,apr,may,JUN,jUl,aug,sep,oct,nov,dec *",
		"1 2 3 4 mon,tue,wed,thu,fri,sat,sun",
		"CRON_TZ=UTC  0 5 * * * *",
		"0 0 1,2,3,5,8,13,21 AUG 3",
		"1-2 3-4 5-6 JUL-sep tUe-FrI",
		"@every 5m",
		"@midnight",
		"@weekly",
		"TZ=UTC  @midnight",
		"TZ=Asia/Tokyo @midnight",
		"@yearly",
		"@annually",
	}
	for _, v := range testcases {
		f.Add(v)
	}
	f.Fuzz(func(t *testing.T, schedule string) {
		parsed, errStd := ParseStandard(schedule)
		if errStd == nil {
			sanityCheck(t, parsed, schedule, "standard parser")
		}
		parsed, errSec := secondParser.Parse(schedule)
		if errSec == nil {
			sanityCheck(t, parsed, schedule, "second parser")
		}
		parsed, errOpt := optionalSecondParser.Parse(schedule)
		if errOpt == nil {
			sanityCheck(t, parsed, schedule, "optional second parser")
		}
	})
}

func sanityCheck(t *testing.T, parsed Schedule, schedule, name string) {
	looksLikeCron(t, schedule, name)
	day, month := getDate(schedule, name)
	if !mightNotExist(day, month) {
		willBeScheduled(t, parsed, schedule, name)
	}
}

func getDate(schedule string, name string) (string, string) {
	fields := getNonTzFields(schedule)
	if len(fields) < 5 {
		return "", ""
	}
	if name == "standard parser" {
		return fields[2], fields[3]
	} else if name == "second parser" {
		return fields[3], fields[4]
	} else if name == "optional second parser" {
		if len(fields) == 6 {
			return fields[3], fields[4]
		} else {
			return fields[2], fields[3]
		}
	}
	return "", ""
}

// mightNotExist filters a supersets of dates that don't exist
func mightNotExist(day, month string) bool {
	return contains(month, day, "2", "feb", "30", "31", "L-30", "L-29") ||
		contains(month, day, "4", "apr", "31", "L-30") ||
		contains(month, day, "6", "jun", "31", "L-30") ||
		contains(month, day, "9", "sep", "31", "L-30") ||
		contains(month, day, "11", "nov", "31", "L-30")
}

func contains(month, day string, monthNum, monthName string, daysTest ...string) bool {
	if !strings.Contains(month, monthNum) && !strings.Contains(strings.ToLower(month), monthName) {
		return false
	}
	for _, d := range daysTest {
		if strings.Contains(day, d) {
			return true
		}
	}
	return false
}

// looksLikeCron checks if the schedule roughly matches expected cron syntax
func looksLikeCron(t *testing.T, schedule, name string) {
	fields := getNonTzFields(schedule)
	if len(fields) == 0 {
		t.Errorf("empty schedule")
	}
	if strings.HasPrefix(fields[0], "@every") {
		if !(len(fields) == 2) {
			t.Errorf("%s %s: expected 2 fields, got %d", name, schedule, len(fields))
		}
	} else if strings.HasPrefix(fields[0], "@") {
		if !(len(fields) == 1) {
			t.Errorf("%s %s: expected 1 field, got %d", name, schedule, len(fields))
		}
	} else {
		if !(len(fields) == 5 || len(fields) == 6) {
			t.Errorf("%s %s: expected 5 or 6 fields, got %d", name, schedule, len(fields))
		}
		validateChars(t, fields, name)
	}
}

func getNonTzFields(schedule string) []string {
	schedule = strings.TrimSpace(schedule)
	fields := strings.Fields(schedule)
	if strings.HasPrefix(fields[0], "TZ=") || strings.HasPrefix(fields[0], "CRON_TZ=") {
		fields = fields[1:]
	}
	return fields
}

func validateChars(t *testing.T, fields []string, name string) {
	for _, field := range fields {
		for _, c := range field {
			if !strings.ContainsRune("0123456789+#*-,/?sunmotewdhfriajbpylgcv", unicode.ToLower(c)) {
				t.Errorf("unexpected character %c in %s for %s", c, fields, name)
			}
		}
	}
}

var startTimes = []time.Time{
	time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	time.Date(2024, 12, 13, 23, 59, 59, 0, time.UTC),
	time.Date(2000, 2, 28, 13, 21, 30, 0, time.UTC),
}

func willBeScheduled(t *testing.T, parsed Schedule, schedule string, name string) {
	for _, startTime := range startTimes {
		next := parsed.Next(startTime)
		if !next.After(startTime) {
			t.Errorf("%s failed for '%s': expected next time to be after %v, got %v", name, schedule, startTime, next)
		}
	}
}
