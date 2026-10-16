package virtualtime_test

import (
	"testing"
	"time"

	"github.com/d-led/phony"
	"github.com/d-led/phony/virtualtime"
)

// Alarm is the smallest actor that waits for time: it rings once, later.
type Alarm struct {
	phony.Inbox
	timer virtualtime.Timer
	rung  bool
	at    time.Time
}

// NewAlarm answers an alarm whose timers come from timer.
func NewAlarm(timer virtualtime.Timer) *Alarm {
	return &Alarm{timer: timer}
}

// RingAfter rings once d has passed.
func (a *Alarm) RingAfter(d time.Duration) {
	a.timer.After(d, func() {
		a.Act(nil, func() {
			a.rung = true
			a.at = a.timer.Now()
		})
	})
}

// Rung is read inside the actor: on the wall clock the actor has a goroutine of
// its own, so the answer is asked of it rather than read from it.
func (a *Alarm) Rung() bool { return a.rung }

func TestWhyWait(t *testing.T) {
	clock := virtualtime.New()
	defer clock.Reset()

	alarm := NewAlarm(clock)

	clock.Block(alarm, func() { alarm.RingAfter(100 * 24 * time.Hour) })

	if alarm.Rung() {
		t.Fatal("the alarm rang before its time")
	}

	before := time.Now()
	clock.Advance(100 * 24 * time.Hour)
	elapsed := time.Since(before)

	if !alarm.Rung() {
		t.Fatal("the alarm did not ring")
	}
	if want := clock.Now(); !alarm.at.Equal(want) {
		t.Fatalf("the alarm rang at %v, want %v", alarm.at, want)
	}
	if elapsed > time.Second {
		t.Fatalf("a hundred days took %v of real time", elapsed)
	}
}
