// Command virtualtime-ticking ticks for three seconds twice: once on the wall
// clock, where the program waits for them, and once on a clock it advances itself,
// where the same three seconds take no time at all.
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
	interval = 100 * time.Millisecond
	threeSec = 3 * time.Second
)

// Ticker ticks every interval until the duration it was asked for has passed.
type Ticker struct {
	phony.Inbox
	timer    virtualtime.Timer
	interval time.Duration
	ticks    int
}

// NewTicker answers a ticker whose timers come from timer: the wall clock in
// production, a virtualtime.Clock in a test.
func NewTicker(timer virtualtime.Timer, interval time.Duration) *Ticker {
	return &Ticker{timer: timer, interval: interval}
}

// TickFor asks the ticker to tick every interval until d has passed, and answers
// the channel the number of ticks arrives on.
func (t *Ticker) TickFor(d time.Duration) <-chan int {
	ticks := make(chan int, 1)

	t.Act(nil, func() {
		left := int(d / t.interval)

		// Declared first, because the last tick cancels it.
		var cancel func() bool
		cancel = t.timer.Every(t.interval, func() {
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
					ticks <- t.ticks
				}
			})
		})
	})

	return ticks
}

// run is a count of ticks and what it cost in wall-clock time.
type run struct {
	ticks   int
	elapsed time.Duration
}

// onTheWallClock ticks for three seconds of real time: this one waits for them.
func onTheWallClock() run {
	ticker := NewTicker(virtualtime.NewReal(), interval)

	start := time.Now()
	ticks := ticker.TickFor(threeSec)
	count := <-ticks // here the three seconds pass

	return run{ticks: count, elapsed: time.Since(start)}
}

// onTheClockTheTestOwns ticks for the same three seconds, which this program
// decides have passed.
func onTheClockTheTestOwns() run {
	clock := virtualtime.New()
	defer clock.Reset()

	ticker := NewTicker(clock, interval)

	start := time.Now()
	ticks := ticker.TickFor(threeSec)
	clock.Advance(threeSec)

	return run{ticks: <-ticks, elapsed: time.Since(start)}
}

func main() {
	waited := onTheWallClock()
	advanced := onTheClockTheTestOwns()

	fmt.Printf("wall clock:   %d ticks in %v\n", waited.ticks, waited.elapsed)
	fmt.Printf("virtual time: %d ticks in %v\n", advanced.ticks, advanced.elapsed)
	fmt.Printf("the same %d ticks, %v sooner\n", advanced.ticks,
		waited.elapsed-advanced.elapsed)
}
