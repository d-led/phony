package main

import (
	"testing"
	"time"

	"github.com/d-led/phony"
	"github.com/d-led/phony/virtualtime"
)

// The example itself, in a size a test suite can afford: both clocks tick every
// ten milliseconds for two hundred milliseconds.
const (
	testInterval = 10 * time.Millisecond
	testFor      = 200 * time.Millisecond
	testWant     = 20
)

func TestTheWallClockTicksInRealTime(t *testing.T) {
	if testing.Short() {
		t.Skip("the point of this test is the real time it takes")
	}

	ticker := NewTicker(virtualtime.NewReal(), testInterval)

	start := time.Now()
	phony.Block(ticker, func() { ticker.TickFor(testFor) })

	// There is nothing to assert yet: wait for the ticks, with a deadline.
	deadline := time.Now().Add(10 * testFor)
	ticks := 0
	for {
		phony.Block(ticker, func() { ticks = ticker.Ticks() })
		if ticks >= testWant || time.Now().After(deadline) {
			break
		}
		time.Sleep(time.Millisecond)
	}

	if ticks != testWant {
		t.Fatalf("ticked %d times in %v, want %d", ticks, testFor, testWant)
	}
	if elapsed := time.Since(start); elapsed < testFor {
		t.Fatalf("the wall clock answered in %v, less than the %v it ticks for", elapsed, testFor)
	}
}

func TestTheVirtualClockTicksWithoutWaiting(t *testing.T) {
	clock := virtualtime.New()
	defer clock.Reset()

	ticker := NewTicker(clock, testInterval)

	start := time.Now()
	clock.Block(ticker, func() { ticker.TickFor(testFor) })
	clock.Advance(testFor)

	ticks := 0
	clock.Block(ticker, func() { ticks = ticker.Ticks() })

	if ticks != testWant {
		t.Fatalf("ticked %d times in %v of virtual time, want %d", ticks, testFor, testWant)
	}
	if elapsed := time.Since(start); elapsed > testFor {
		t.Fatalf("advancing %v took %v of real time", testFor, elapsed)
	}
}

// The example's own functions, so that what the README shows is what runs here.
func TestTheSameTicksBothWays(t *testing.T) {
	if testing.Short() {
		t.Skip("the wall-clock half takes its time on purpose")
	}

	if waited := onTheWallClock(); waited.ticks != want {
		t.Fatalf("the wall clock ticked %d times, want %d", waited.ticks, want)
	}
	if advanced := onTheClockTheTestOwns(); advanced.ticks != want {
		t.Fatalf("the virtual clock ticked %d times, want %d", advanced.ticks, want)
	}
}
