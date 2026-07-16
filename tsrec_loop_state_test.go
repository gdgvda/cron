package cron

import (
	"testing"
	"time"
)

func TestTSRECLoopStateBeginOnceUntilEnd(t *testing.T) {
	state := &tsrecLoopState{}

	if !state.begin() {
		t.Fatal("expected begin to succeed on an inactive loop")
	}
	if state.begin() {
		t.Error("expected begin to fail while a goroutine is active")
	}

	state.end()
	if !state.begin() {
		t.Error("expected begin to succeed again after end")
	}
	state.end()
}

func TestTSRECLoopStateInterruptStopsGoroutine(t *testing.T) {
	state := &tsrecLoopState{}
	state.begin()

	exited := make(chan struct{})
	go func() {
		for state.await(nil) {
		}
		// exited is closed before end so that interrupt returning proves the goroutine exited.
		close(exited)
		state.end()
	}()

	state.interrupt()
	select {
	case <-exited:
	default:
		t.Error("expected interrupt to return only after the goroutine ended")
	}
}

func TestTSRECLoopStateInterruptAndAwaitEndWhenInactive(t *testing.T) {
	state := &tsrecLoopState{}
	done := make(chan struct{})
	go func() {
		state.interrupt()
		state.awaitEnd()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("expected interrupt and awaitEnd to return immediately on an inactive loop")
	}
}

func TestTSRECLoopStateAwaitEnd(t *testing.T) {
	state := &tsrecLoopState{}
	state.begin()

	finish := make(chan struct{})
	go func() {
		<-finish
		state.end()
	}()

	done := make(chan struct{})
	go func() {
		state.awaitEnd()
		close(done)
	}()

	select {
	case <-done:
		t.Fatal("expected awaitEnd to block while the goroutine is active")
	case <-time.After(30 * time.Millisecond):
	}

	close(finish)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("expected awaitEnd to return once the goroutine ended")
	}
}

func TestTSRECLoopStateAwaitDue(t *testing.T) {
	state := &tsrecLoopState{}
	state.begin()

	wallTimer := time.NewTimer(time.Millisecond)
	defer wallTimer.Stop()
	if !state.await(wallTimer.C) {
		t.Error("expected await to keep running on wall-clock timer expiry")
	}
	state.end()
}

func TestTSRECLoopStateNudge(t *testing.T) {
	state := &tsrecLoopState{}
	state.nudge() // inactive: dropped, must not panic

	state.begin()
	state.nudge()
	state.nudge()

	// One signal is pending: await returns at once.
	if !state.await(nil) {
		t.Fatal("expected await to keep running after a nudge")
	}

	// Consecutive nudges coalesce into that one signal: a second await must block.
	done := make(chan struct{})
	go func() {
		state.await(nil)
		close(done)
	}()
	select {
	case <-done:
		t.Error("expected consecutive nudges to coalesce into one pending signal")
	case <-time.After(30 * time.Millisecond):
	}

	// Release the waiting goroutine and shut down.
	state.nudge()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("expected await to return after a fresh nudge")
	}
	state.end()
}
