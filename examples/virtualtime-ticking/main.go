// Command virtualtime-ticking ticks for three seconds twice: once on the wall
// clock, where the program really waits, and once on a clock it advances itself,
// where the same three seconds take no time at all.
//
//	go run .
//
// It is the example of the README's "Testing actors that wait for time", as a
// dependency: this module imports github.com/d-led/phony.
package main

import (
	"fmt"
	"time"

	"github.com/d-led/phony"
	"github.com/d-led/phony/virtualtime"
)

const (
	interval = 100 * time.Millisecond
	threeSec = 3 * time.Second
	want     = 30
)

// Ticker ticks every interval until the duration it was asked for has passed.
type Ticker struct {
	phony.Inbox
	timer    virtualtime.Timer
	interval time.Duration
	ticks    int
}

// NewTicker answers a ticker whose timers come from timer: a wall-clock Timer
// here, a virtual clock in the second half of the example.
func NewTicker(timer virtualtime.Timer, interval time.Duration) *Ticker {
	return &Ticker{timer: timer, interval: interval}
}

// TickFor ticks every interval until d has passed: thirty ticks for three
// seconds of a hundred-millisecond interval.
func (t *Ticker) TickFor(d time.Duration) {
	left := int(d / t.interval)

	// Declared first, because the last tick cancels it.
	var cancel func() bool
	cancel = t.timer.Every(t.interval, func() {
		t.Act(nil, func() {
			if left <= 0 {
				cancel()
				return
			}
			left--
			t.ticks++
		})
	})
}

// Ticks is the count, read inside the actor.
func (t *Ticker) Ticks() int { return t.ticks }

// run answers the number of ticks and what the run cost in wall-clock time.
type run struct {
	ticks   int
	elapsed time.Duration
}

// onTheWallClock ticks for three seconds of real time. The program has nothing
// to do but wait, and the count has to be asked of the actor with a deadline.
func onTheWallClock() run {
	ticker := NewTicker(virtualtime.NewReal(), interval)

	start := time.Now()
	phony.Block(ticker, func() { ticker.TickFor(threeSec) })

	deadline := time.Now().Add(2 * threeSec)
	ticks := 0
	for {
		phony.Block(ticker, func() { ticks = ticker.Ticks() })
		if ticks >= want || time.Now().After(deadline) {
			break
		}
		time.Sleep(time.Millisecond)
	}

	return run{ticks: ticks, elapsed: time.Since(start)}
}

// onTheClockTheTestOwns ticks for the same three seconds, which the program
// decides have passed.
func onTheClockTheTestOwns() run {
	clock := virtualtime.New()
	defer clock.Reset()

	ticker := NewTicker(clock, interval)

	start := time.Now()
	clock.Block(ticker, func() { ticker.TickFor(threeSec) })
	clock.Advance(threeSec)

	ticks := 0
	clock.Block(ticker, func() { ticks = ticker.Ticks() })

	return run{ticks: ticks, elapsed: time.Since(start)}
}

func main() {
	waited := onTheWallClock()
	advanced := onTheClockTheTestOwns()

	fmt.Printf("wall clock:   %d ticks in %v\n", waited.ticks, waited.elapsed)
	fmt.Printf("virtual time: %d ticks in %v\n", advanced.ticks, advanced.elapsed)
	fmt.Printf("the same %d ticks, %v sooner\n", advanced.ticks,
		waited.elapsed-advanced.elapsed)
}
