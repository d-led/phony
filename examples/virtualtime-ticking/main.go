// Command virtualtime-ticking runs the same three ticks twice: on the wall clock,
// where the three seconds have to pass, and on a clock this program owns, where the
// program says they have passed and they cost nothing.
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

// tickEvery is how often the ticker ticks; the runs below tick three times.
const tickEvery = time.Second

// Ticker ticks once per tickEvery until the ticking it was asked for is over.
type Ticker struct {
	phony.Inbox
	timer    virtualtime.Timer
	ticks    int
	ticked   chan int // where the actor reports each count
	reported int      // the last count this program read
}

// NewTicker answers a ticker whose time comes from timer: the wall clock in
// production, a virtualtime.Clock in a test. A ticker ticks once, so a second run
// needs a second ticker.
func NewTicker(timer virtualtime.Timer) *Ticker {
	return &Ticker{timer: timer}
}

// TickTimesAsync asks the ticker to tick n times, and does not wait for them: the
// actor reports each count on its channel, and does not ask anyone to move time.
func (t *Ticker) TickTimesAsync(n int) {
	t.ticked = make(chan int, n) // room for every count, so the actor never waits

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
				t.ticked <- t.ticks

				if left == 0 {
					cancel()
				}
			})
		})
	})
}

// GetTicks answers the last count the ticker reported, having taken whatever counts
// were waiting.
//
// The count comes from the channel and not from the actor's own field because on the
// wall clock the actor writes it on a thread of its own. On a clock this program
// owns the actor runs on this thread, so by the time the program says the time has
// passed, the count is waiting here.
func (t *Ticker) GetTicks() int {
	for {
		select {
		case t.reported = <-t.ticked:
		default:
			return t.reported
		}
	}
}

// RealTime is the wall clock: the time passes by itself, so waiting for it is
// waiting for it.
func RealTime() *virtualtime.Real { return virtualtime.NewReal() }

// VirtualTime is a clock this program owns: the program says how much time has
// passed, so the same three seconds cost nothing. Reset gives back the clock the
// program had before; a test does that with defer.
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
		wall := RealTime()
		ticker := NewTicker(wall)
		ticker.TickTimesAsync(3)

		// Waiting for the wall clock is waiting: the three seconds have to pass.
		wall.WaitFor(10*time.Second, func() bool { return ticker.GetTicks() >= 3 })

		return ticker.GetTicks()
	})
}

// onTheClockTheProgramOwns ticks three times on a clock this program owns: the same
// three seconds, which cost nothing, because the program says they have passed.
func onTheClockTheProgramOwns() (int, time.Duration) {
	return timeIt(func() int {
		clock := VirtualTime()
		defer clock.Reset()

		ticker := NewTicker(clock)
		ticker.TickTimesAsync(3)

		// The program owns the clock, so the program says the three seconds have
		// passed. This is the line the wall clock cannot have.
		clock.Advance(3 * tickEvery)

		return ticker.GetTicks()
	})
}

func main() {
	realTicks, realCost := onTheWallClock()
	virtualTicks, virtualCost := onTheClockTheProgramOwns()

	fmt.Printf("real time:    %d ticks in %v\n", realTicks, realCost)
	fmt.Printf("virtual time: %d ticks in %v\n", virtualTicks, virtualCost)
}
