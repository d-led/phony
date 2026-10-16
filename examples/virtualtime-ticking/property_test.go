package main

import (
	"testing"
	"testing/quick"
	"time"

	"github.com/d-led/phony/virtualtime"
)

// The property of the example actor: it ticks as often as it was asked to and no
// more, and the clock the program owns moves by exactly that many ticks — three
// seconds for three ticks of a second, whatever the number is.
func TestTheTickerTicksAsOftenAsItWasAskedTo(t *testing.T) {
	property := func(raw int8) bool {
		want := ticksFrom(raw)

		clock := virtualtime.New()
		defer clock.Reset()

		ticker := NewTicker(clock)

		start := clock.Now()
		ticker.TickTimesAsync(want)
		ticker.WaitForTicksAtLeast(want, time.Duration(want)*tickEvery+tickEvery)

		return ticker.GetTicks() == want && clock.Now().Sub(start) == time.Duration(want)*tickEvery
	}

	if err := quick.Check(property, &quick.Config{MaxCount: 200}); err != nil {
		t.Error(err)
	}
}

// ticksFrom answers a number of ticks from 1 to 64, from a generated value.
func ticksFrom(raw int8) int {
	if raw < 0 {
		raw = -raw
	}

	return int(raw)%64 + 1
}
