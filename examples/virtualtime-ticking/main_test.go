package main

import (
	"testing"
	"time"
)

// The example's own runs, which is what the README shows: the same three ticks on
// the wall clock, where the three seconds have to pass, and on a clock the program
// owns, where the program says they have passed.
func TestTheWallClockTicksInThreeSeconds(t *testing.T) {
	if testing.Short() {
		t.Skip("the point of this test is the three seconds")
	}

	ticks, cost := onTheWallClock()

	if ticks != 3 {
		t.Fatalf("the wall clock ticked %d times, want 3", ticks)
	}
	if cost < 3*tickEvery {
		t.Fatalf("the wall clock cost %v, less than the %v of ticks it was asked for", cost, 3*tickEvery)
	}
}

func TestTheSameTicksCostNothingOnAClockTheProgramOwns(t *testing.T) {
	ticks, cost := onTheClockTheProgramOwns()

	if ticks != 3 {
		t.Fatalf("the clock this program owns ticked %d times, want 3", ticks)
	}
	if cost > tickEvery {
		t.Fatalf("the ticks took %v of real time: the clock this program owns does not wait for them", cost)
	}
}

// The ticks are the time the program said had passed, and they are not early: the
// clock moves when the program advances it, and not a nanosecond before.
func TestTheTicksAreTheTimeTheProgramSaysHasPassed(t *testing.T) {
	clock := VirtualTime()
	defer clock.Reset()

	ticker := NewTicker(clock)
	ticker.TickTimesAsync(3)

	clock.Advance(3*tickEvery - time.Nanosecond) // a nanosecond short of the third tick
	if ticks := ticker.GetTicks(); ticks != 2 {
		t.Fatalf("the clock is a nanosecond short of the third tick and the ticker reports %d ticks, want 2", ticks)
	}

	clock.Advance(time.Nanosecond) // the third tick, on the instant
	if ticks := ticker.GetTicks(); ticks != 3 {
		t.Fatalf("the clock reached the third tick and the ticker reports %d ticks, want 3", ticks)
	}
}
