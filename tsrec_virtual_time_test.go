package cron

import (
	"sync"
	"testing"
	"time"
)

func TestTSRECVirtualTimeFrozenByDefault(t *testing.T) {
	start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	v := newTSRECVirtualTime(start)

	time.Sleep(20 * time.Millisecond)

	if !v.now().Equal(start) {
		t.Errorf("expected time frozen at %v, got %v", start, v.now())
	}
}

func TestTSRECVirtualTimeAdvanceMovesForwardOnly(t *testing.T) {
	start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	v := newTSRECVirtualTime(start)

	future := start.Add(time.Hour)
	v.advance(future)
	if !v.now().Equal(future) {
		t.Errorf("expected time advanced to %v, got %v", future, v.now())
	}

	v.advance(start)
	if !v.now().Equal(future) {
		t.Errorf("expected advance never to move time backwards, got %v", v.now())
	}
}

func TestTSRECVirtualTimeFlowTracksWallClock(t *testing.T) {
	start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	v := newTSRECVirtualTime(start)

	v.flow()
	const sleep = 50 * time.Millisecond
	time.Sleep(sleep)

	elapsed := v.now().Sub(start)
	if elapsed < sleep/2 {
		t.Errorf("expected flowing time to track the wall clock, only %v elapsed", elapsed)
	}
	if elapsed > 10*sleep {
		t.Errorf("flowing time ran too fast: %v elapsed", elapsed)
	}
}

func TestTSRECVirtualTimeAdvanceIgnoredWhileFlowing(t *testing.T) {
	start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	v := newTSRECVirtualTime(start)

	v.flow()
	v.advance(start.Add(time.Hour))

	if v.now().Sub(start) >= time.Hour {
		t.Errorf("expected advance to be a no-op while flowing, got %v", v.now())
	}
}

func TestTSRECVirtualTimeFreezePinsCurrentInstant(t *testing.T) {
	start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	v := newTSRECVirtualTime(start)

	v.flow()
	time.Sleep(20 * time.Millisecond)
	v.freeze()

	pinned := v.now()
	if pinned.Before(start) {
		t.Errorf("expected frozen time at or after %v, got %v", start, pinned)
	}
	time.Sleep(20 * time.Millisecond)
	if !v.now().Equal(pinned) {
		t.Errorf("expected time to stay pinned at %v after freeze, got %v", pinned, v.now())
	}
}

func TestTSRECVirtualTimeConcurrentUse(t *testing.T) {
	start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	v := newTSRECVirtualTime(start)

	var wg sync.WaitGroup
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				switch (g + i) % 4 {
				case 0:
					v.now()
				case 1:
					v.advance(start.Add(time.Duration(i) * time.Second))
				case 2:
					v.flow()
				case 3:
					v.freeze()
				}
			}
		}(g)
	}
	wg.Wait()

	v.freeze()
	if v.now().Before(start) {
		t.Errorf("expected time never to move before %v, got %v", start, v.now())
	}
}
