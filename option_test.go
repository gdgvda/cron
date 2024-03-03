package cron

import (
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestWithLocation(t *testing.T) {
	c := New(WithLocation(time.UTC))
	if c.location != time.UTC {
		t.Errorf("expected UTC, got %v", c.location)
	}
}

func TestWithParser(t *testing.T) {
	var parser = NewParser(Dow)
	c := New(WithParser(parser))
	if c.parser != parser {
		t.Error("expected provided parser")
	}
}

func TestWithVerboseLogger(t *testing.T) {
	var buf syncWriter
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	c := New(WithLogger(logger))
	if c.logger != logger {
		t.Error("expected provided logger")
	}

	_, err := c.Add("@every 1s", func() {})
	if err != nil {
		t.Error("non-nil error")
	}
	c.Start()
	time.Sleep(OneSecond)
	c.Stop()
	out := buf.String()
	if !strings.Contains(out, "start") ||
		!strings.Contains(out, "run") {
		t.Error("expected to see some actions, got:", out)
	}
}
