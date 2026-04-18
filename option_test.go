package cron

import (
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestWithClock(t *testing.T) {
	clock := NewDefaultClock(time.UTC, DefaultNopTimer)
	c := New(WithClock(clock))
	if c.clock != clock {
		t.Error("expected provided clock")
	}
}

func TestWithVerboseLogger(t *testing.T) {
	var buf syncWriter
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	c := New(WithLogger(logger))
	if c.logger != logger {
		t.Error("expected provided logger")
	}

	sched, err := ParseStandard("@every 1s")
	if err != nil {
		t.Error("non-nil error")
	}
	_, err = c.Schedule(sched, func() {})
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

func TestWithOnCycleCompleted(t *testing.T) {
	var buf syncWriter
	f := func() {
		_, err := buf.Write([]byte{'E', 'N', 'D'})
		if err != nil {
			t.Error("non-nil error")
		}
	}
	c := New(WithOnCycleCompleted(f))

	sched, err := ParseStandard("@every 1s")
	if err != nil {
		t.Error("non-nil error")
	}
	_, err = c.Schedule(sched, func() {
		_, err := buf.Write([]byte{'1'})
		if err != nil {
			t.Error("non-nil error")
		}
	})
	if err != nil {
		t.Error("non-nil error")
	}
	_, err = c.Schedule(sched, func() {
		_, err := buf.Write([]byte{'2'})
		if err != nil {
			t.Error("non-nil error")
		}
	})
	if err != nil {
		t.Error("non-nil error")
	}
	c.Start()
	time.Sleep(OneSecond)
	c.Stop()
	out := buf.String()
	if !strings.HasSuffix(out, "END") {
		t.Error("expected callback to be executed after jobs completed")
	}
}
