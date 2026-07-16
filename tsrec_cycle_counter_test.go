package cron

import (
	"sync"
	"testing"
	"time"
)

func TestTSRECCycleCounterCounts(t *testing.T) {
	c := newTSRECCycleCounter()
	if c.running() != 0 {
		t.Errorf("expected 0 running cycles, got %d", c.running())
	}

	c.started()
	c.started()
	if c.running() != 2 {
		t.Errorf("expected 2 running cycles, got %d", c.running())
	}

	c.completed()
	if c.running() != 1 {
		t.Errorf("expected 1 running cycle, got %d", c.running())
	}
}

func TestTSRECCycleCounterAwaitNoneReturnsImmediatelyAtZero(t *testing.T) {
	c := newTSRECCycleCounter()
	done := make(chan struct{})
	go func() {
		c.awaitNone()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("expected awaitNone to return immediately when no cycle is running")
	}
}

func TestTSRECCycleCounterAwaitNoneBlocksWhileRunning(t *testing.T) {
	c := newTSRECCycleCounter()
	c.started()

	done := make(chan struct{})
	go func() {
		c.awaitNone()
		close(done)
	}()

	select {
	case <-done:
		t.Fatal("expected awaitNone to block while a cycle is running")
	case <-time.After(30 * time.Millisecond):
	}

	c.completed()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("expected awaitNone to return once no cycle is running")
	}
}

func TestTSRECCycleCounterConcurrentUse(t *testing.T) {
	c := newTSRECCycleCounter()
	var wg sync.WaitGroup
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				c.started()
				c.completed()
			}
		}()
	}
	wg.Wait()

	c.awaitNone()
	if c.running() != 0 {
		t.Errorf("expected 0 running cycles after balanced starts and completions, got %d", c.running())
	}
}
