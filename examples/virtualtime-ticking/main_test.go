package main

import (
	"testing"
	"time"

	"github.com/d-led/phony/virtualtime"
)

// The example in a size a test suite can afford: twenty ticks of ten milliseconds,
// which is two hundred milliseconds either way.
const (
	want         = 30 // ticks of the three seconds the example runs
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
	ticks := ticker.TickFor(testFor)
	count := <-ticks // here the two hundred milliseconds pass

	if count != testWant {
		t.Fatalf("ticked %d times in %v, want %d", count, testFor, testWant)
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
	ticks := ticker.TickFor(testFor)
	clock.Advance(testFor)

	if count := <-ticks; count != testWant {
		t.Fatalf("ticked %d times in %v of virtual time, want %d", count, testFor, testWant)
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
