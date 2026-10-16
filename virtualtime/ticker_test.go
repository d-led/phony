package virtualtime_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/d-led/phony"
	"github.com/d-led/phony/virtualtime"
)

// This file is the ticking example of the README and of the package
// documentation: one actor and the same three seconds, on the wall clock where it
// waits and on a clock the test advances, where it does not.

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

// NewTicker answers a ticker whose timers come from timer: the wall clock in
// production, a virtualtime.Clock in a test.
func NewTicker(timer virtualtime.Timer, interval time.Duration) *Ticker {
	return &Ticker{timer: timer, interval: interval}
}

// TickFor asks the ticker to tick every interval until d has passed, and answers
// the channel the number of ticks arrives on. The channel has room for the number,
// so the ticker never waits for whoever reads it.
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

func TestTickingOnTheWallClockTakesThreeSeconds(t *testing.T) {
	if testing.Short() {
		t.Skip("the point of this test is the three seconds")
	}

	ticker := NewTicker(virtualtime.NewReal(), interval)

	start := time.Now()
	ticks := ticker.TickFor(threeSec)
	count := <-ticks // here the three seconds pass

	if count != want {
		t.Fatalf("ticked %d times in three seconds, want %d", count, want)
	}
	if elapsed := time.Since(start); elapsed < threeSec {
		t.Fatalf("the wall clock answered in %v, less than the %v it ticks for", elapsed, threeSec)
	}

	t.Logf("wall clock: %d ticks in %v", count, time.Since(start))
}

func TestTickingOnTheVirtualClockTakesNoTime(t *testing.T) {
	clock := virtualtime.New()
	defer clock.Reset()

	ticker := NewTicker(clock, interval)

	start := time.Now()
	ticks := ticker.TickFor(threeSec)
	clock.Advance(threeSec)

	count := <-ticks // the count is there: the test decided the time had passed

	if count != want {
		t.Fatalf("ticked %d times in three seconds, want %d", count, want)
	}
	if elapsed := time.Since(start); elapsed > threeSec {
		t.Fatalf("advancing %v took %v of real time", threeSec, elapsed)
	}

	t.Logf("virtual time: %d ticks in %v", count, time.Since(start))
}

func TestTickingForAHundredDays(t *testing.T) {
	clock := virtualtime.New()
	defer clock.Reset()

	ticker := NewTicker(clock, 24*time.Hour)

	ticks := ticker.TickFor(100 * 24 * time.Hour)
	clock.Advance(100 * 24 * time.Hour)

	if count := <-ticks; count != 100 {
		t.Fatalf("ticked %d times in a hundred days, want 100", count)
	}
}

func TestNothingIsLeftRunningWhenTheTickingIsOver(t *testing.T) {
	clock := virtualtime.New()
	defer clock.Reset()

	ticker := NewTicker(clock, interval)

	ticker.TickFor(threeSec)
	clock.Advance(threeSec)

	if timers, ready := clock.Pending(); timers != 0 || ready != 0 {
		t.Fatalf("pending: %d timers, %d ready; want the ticking to be over and nothing running", timers, ready)
	}
}

// Example is the ticking test of the package documentation: thirty ticks of a
// hundred milliseconds, for three seconds the test decides have passed. The
// wall-clock half of the same program, which does take three seconds, is
// examples/virtualtime-ticking.
func Example() {
	clock := virtualtime.New()
	defer clock.Reset()

	ticker := NewTicker(clock, interval)

	start := clock.Now()
	ticks := ticker.TickFor(threeSec)
	clock.Advance(threeSec)

	fmt.Println("ticks:", <-ticks)
	fmt.Println("the clock moved by", clock.Now().Sub(start))

	// Output:
	// ticks: 30
	// the clock moved by 3s
}
