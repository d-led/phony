// Command virtualtime-ticking runs the same three ticks twice: on the wall clock,
// where the three seconds have to pass, and on a clock this program owns, where
// the same three seconds cost nothing.
//
// It is the ticking example of the README and of the virtualtime package, as a
// program that depends on this module:
//
//	go run .
package main

import (
	"fmt"
	"time"

	"github.com/d-led/phony"
	"github.com/d-led/phony/virtualtime"
)

const (
	tickEvery = time.Second
	three     = 3
)

// Ticker ticks once per tickEvery until the ticking it was asked for is over.
type Ticker struct {
	phony.Inbox
	timer    virtualtime.Timer
	ticks    int
	ticked   chan int // what the actor reports when the ticking is over
	reported int      // what the program has been told, written by the waits
}

// NewTicker answers a ticker whose time comes from timer: the wall clock in
// production, a virtualtime.Clock in a test.
func NewTicker(timer virtualtime.Timer) *Ticker {
	return &Ticker{timer: timer, ticked: make(chan int, 1)}
}

// TickTimesAsync asks the ticker to tick n times, and does not wait for them: the
// count arrives on the ticker's channel when the ticking is over.
func (t *Ticker) TickTimesAsync(n int) {
	t.Act(nil, func() {
		left := n

		// Declared first, because the last tick cancels it.
		var cancel func() bool
		cancel = t.timer.Every(tickEvery, func() {
			t.Act(nil, func() {
				// The ticker of the wall clock may have one tick in flight after
				// the last one was counted.
				if left <= 0 {
					cancel()
					return
				}

				left--
				t.ticks++

				if left == 0 {
					cancel()
					t.ticked <- t.ticks
				}
			})
		})
	})
}

// WaitForTicksAtLeast lets the clock pass the time until the ticker has reported
// n ticks, giving up after at most d.
//
// Waiting is waiting on the wall clock, and it is the program saying that the time
// has passed on a clock the program owns: this line is the same in both, and only
// the clock changes. What the ticks came to is in GetTicks.
func (t *Ticker) WaitForTicksAtLeast(n int, d time.Duration) {
	t.timer.WaitFor(d, func() bool {
		select {
		case t.reported = <-t.ticked:
		default:
		}

		return t.reported >= n
	})
}

// GetTicks answers the number of ticks the ticker has reported to the program,
// rather than letting anyone read the actor's own count: on the wall clock the
// actor writes it on a thread of its own.
func (t *Ticker) GetTicks() int { return t.reported }

// RealTime is the wall clock: the time passes by itself, so waiting for it is
// waiting for it.
func RealTime() virtualtime.Timer { return virtualtime.NewReal() }

// VirtualTime is a clock this program owns: the program says how much time has
// passed, so the same wait costs nothing. Reset gives back the clock the program
// had before; a test does that with defer.
func VirtualTime() *virtualtime.Clock { return virtualtime.New() }

// timeIt runs what it is given, and answers the number of ticks the run produced
// and what the run cost in wall-clock time.
func timeIt(run func() int) (int, time.Duration) {
	start := time.Now()
	ticks := run()

	return ticks, time.Since(start)
}

// onTheWallClock ticks three times on the wall clock: three seconds of somebody's
// life, which the program can do nothing about.
func onTheWallClock() (int, time.Duration) {
	return timeIt(func() int {
		ticker := NewTicker(RealTime())
		ticker.TickTimesAsync(three)
		ticker.WaitForTicksAtLeast(three, 10*time.Second)

		return ticker.GetTicks()
	})
}

// onTheClockTheProgramOwns ticks three times on a clock this program owns: the
// same three seconds, which cost nothing, because the program says they have
// passed.
func onTheClockTheProgramOwns() (int, time.Duration) {
	return timeIt(func() int {
		clock := VirtualTime()
		defer clock.Reset()

		ticker := NewTicker(clock)
		ticker.TickTimesAsync(three)
		ticker.WaitForTicksAtLeast(three, 10*time.Second)

		return ticker.GetTicks()
	})
}

func main() {
	realTicks, realCost := onTheWallClock()
	virtualTicks, virtualCost := onTheClockTheProgramOwns()

	fmt.Printf("real time:    %d ticks in %v\n", realTicks, realCost)
	fmt.Printf("virtual time: %d ticks in %v\n", virtualTicks, virtualCost)
}
