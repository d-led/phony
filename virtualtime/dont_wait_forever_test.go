package virtualtime_test

import (
	"testing"
	"time"

	"github.com/Arceliar/phony"
	"github.com/Arceliar/phony/virtualtime"
)

// Alarm is a phony actor whose timers come from a Timer, which is the whole
// point: the same actor runs on the wall clock in production and on a clock the
// test advances.
type Alarm struct {
	phony.Inbox
	timer virtualtime.Timer
	rung  bool
	at    time.Time
}

func NewAlarm(timer virtualtime.Timer) *Alarm {
	return &Alarm{timer: timer}
}

func (a *Alarm) RingAfter(d time.Duration) {
	a.timer.After(d, func() {
		a.Act(nil, func() {
			a.rung = true
			a.at = a.timer.Now()
		})
	})
}

func (a *Alarm) Rung() bool { return a.rung }

// Ticker counts its own ticks, one per interval.
type Ticker struct {
	phony.Inbox
	timer virtualtime.Timer
	ticks int
}

func NewTicker(timer virtualtime.Timer) *Ticker {
	return &Ticker{timer: timer}
}

func (t *Ticker) TickEvery(interval time.Duration) {
	t.timer.Every(interval, func() {
		t.Act(nil, func() { t.ticks++ })
	})
}

func (t *Ticker) Ticks() int { return t.ticks }

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

func TestTickingSome(t *testing.T) {
	clock := virtualtime.New()
	defer clock.Reset()

	ticker := NewTicker(clock)

	clock.Block(ticker, func() { ticker.TickEvery(24 * time.Hour) })

	clock.Advance(100 * 24 * time.Hour)

	if got, want := ticker.Ticks(), 100; got != want {
		t.Fatalf("ticks after a hundred days: %d, want %d", got, want)
	}

	clock.Advance(time.Second)

	if got, want := ticker.Ticks(), 100; got != want {
		t.Fatalf("ticks after one more second: %d, want %d", got, want)
	}
}

func TestNothingRunsBehindTheTestsBack(t *testing.T) {
	clock := virtualtime.New()
	defer clock.Reset()

	ticker := NewTicker(clock)

	clock.Block(ticker, func() { ticker.TickEvery(time.Hour) })
	clock.Advance(time.Hour)

	timers, ready := clock.Pending()
	if timers != 1 || ready != 0 {
		t.Fatalf("pending: %d timers, %d ready; want 1 timer and nothing ready", timers, ready)
	}
}
