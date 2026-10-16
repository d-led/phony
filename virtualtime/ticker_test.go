package virtualtime_test

import (
	"testing"
	"time"

	"github.com/Arceliar/phony"
	"github.com/Arceliar/phony/virtualtime"
)

// This file is the "show me the code" section of the README: the actor, and the
// same test run on the wall clock and on a clock the test advances.

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

// NewTicker answers a ticker whose timers come from timer: a wall-clock Timer in
// production, a virtualtime.Clock in a test.
func NewTicker(timer virtualtime.Timer, interval time.Duration) *Ticker {
	return &Ticker{timer: timer, interval: interval}
}

// TickFor ticks every interval until d has passed: a hundred-millisecond
// interval for three seconds is thirty ticks, whether the time came from waiting
// or from a clock the test advanced.
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

// Ticks is the count, read inside the actor: on the wall clock the actor has a
// goroutine of its own, so the count is asked of it rather than read from it.
func (t *Ticker) Ticks() int { return t.ticks }

func TestTickingOnTheWallClockTakesThreeSeconds(t *testing.T) {
	if testing.Short() {
		t.Skip("the point of this test is the three seconds")
	}

	ticker := NewTicker(virtualtime.NewReal(), interval)

	start := time.Now()
	phony.Block(ticker, func() { ticker.TickFor(threeSec) })

	// There is nothing to assert yet, so wait for the ticks with a deadline.
	deadline := time.Now().Add(2 * threeSec)
	var ticks int
	for {
		phony.Block(ticker, func() { ticks = ticker.Ticks() })
		if ticks >= want || time.Now().After(deadline) {
			break
		}
		time.Sleep(time.Millisecond)
	}

	if ticks != want {
		t.Fatalf("ticked %d times in three seconds, want %d", ticks, want)
	}
	t.Logf("wall clock: %d ticks in %v", ticks, time.Since(start))
}

func TestTickingOnTheVirtualClockTakesNoTime(t *testing.T) {
	clock := virtualtime.New()
	defer clock.Reset()

	ticker := NewTicker(clock, interval)

	start := time.Now()
	clock.Block(ticker, func() { ticker.TickFor(threeSec) })
	clock.Advance(threeSec)

	ticks := 0
	clock.Block(ticker, func() { ticks = ticker.Ticks() })

	if ticks != want {
		t.Fatalf("ticked %d times in three seconds, want %d", ticks, want)
	}
	t.Logf("virtual time: %d ticks in %v", ticks, time.Since(start))
}

func TestTickingForAHundredDays(t *testing.T) {
	clock := virtualtime.New()
	defer clock.Reset()

	ticker := NewTicker(clock, 24*time.Hour)

	clock.Block(ticker, func() { ticker.TickFor(100 * 24 * time.Hour) })
	clock.Advance(100 * 24 * time.Hour)

	ticks := 0
	clock.Block(ticker, func() { ticks = ticker.Ticks() })

	if ticks != 100 {
		t.Fatalf("ticked %d times in a hundred days, want 100", ticks)
	}
}

func TestNothingRunsBehindTheTestsBack(t *testing.T) {
	clock := virtualtime.New()
	defer clock.Reset()

	ticker := NewTicker(clock, time.Hour)

	clock.Block(ticker, func() { ticker.TickFor(time.Hour) })
	clock.Advance(time.Hour)

	if timers, ready := clock.Pending(); timers != 1 || ready != 0 {
		t.Fatalf("pending: %d timers, %d ready; want the one timer and nothing ready", timers, ready)
	}
}
