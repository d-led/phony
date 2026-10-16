package virtualtime_test

import (
	"testing"
	"time"

	"github.com/d-led/phony"
	"github.com/d-led/phony/virtualtime"
)

// This file is the "why wait at all" test: an actor that rings in a hundred days,
// and a test that decides the hundred days have passed.

// Alarm is the smallest actor that waits for time: it rings once, later.
type Alarm struct {
	phony.Inbox
	timer virtualtime.Timer
}

// NewAlarm answers an alarm whose timers come from timer: the wall clock in
// production, a virtualtime.Clock in a test.
func NewAlarm(timer virtualtime.Timer) *Alarm {
	return &Alarm{timer: timer}
}

// RingAfter asks the alarm to ring once d has passed, and answers the channel the
// instant it rang at arrives on. The channel has room for the instant, so the alarm
// never waits for whoever reads it.
func (a *Alarm) RingAfter(d time.Duration) <-chan time.Time {
	rang := make(chan time.Time, 1)

	a.Act(nil, func() {
		a.timer.After(d, func() {
			a.Act(nil, func() { rang <- a.timer.Now() })
		})
	})

	return rang
}

func TestWhyWait(t *testing.T) {
	clock := virtualtime.New()
	defer clock.Reset()

	alarm := NewAlarm(clock)

	before := time.Now()
	rang := alarm.RingAfter(100 * 24 * time.Hour)

	clock.Advance(100 * 24 * time.Hour)

	if elapsed := time.Since(before); elapsed > time.Second {
		t.Fatalf("a hundred days took %v of real time", elapsed)
	}
	if at := <-rang; !at.Equal(clock.Now()) {
		t.Fatalf("the alarm rang at %v, want %v", at, clock.Now())
	}
}
