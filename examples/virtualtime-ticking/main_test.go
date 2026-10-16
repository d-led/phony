package main

import (
	"testing"
)

// The example's own runs, which is what the README shows: the same three ticks on
// the wall clock and on a clock this program owns.
func TestTheWallClockTicksInThreeSeconds(t *testing.T) {
	if testing.Short() {
		t.Skip("the point of this test is the three seconds")
	}

	ticks, cost := onTheWallClock()

	if ticks != three {
		t.Fatalf("ticked %d times, want %d", ticks, three)
	}
	if cost < three*tickEvery {
		t.Fatalf("the wall clock cost %v, less than the %v of ticks it was asked for", cost, three*tickEvery)
	}
}

func TestTheSameTicksCostNothingOnAClockTheProgramOwns(t *testing.T) {
	ticks, cost := onTheClockTheProgramOwns()

	if ticks != three {
		t.Fatalf("ticked %d times, want %d", ticks, three)
	}
	if cost > tickEvery {
		t.Fatalf("the ticks took %v of real time: the clock this program owns does not wait for them", cost)
	}
}
