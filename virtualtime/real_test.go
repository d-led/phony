package virtualtime_test

import (
	"testing"
	"time"

	"github.com/d-led/phony/virtualtime"
)

// An actor written against a Timer does not know which one it was given, so the
// wall-clock Timer is tested to the same contract as the clock: it calls back
// when it is due, it reports a cancel, and it answers the time it is keeping.
func TestTheRealTimerCallsBack(t *testing.T) {
	wall := virtualtime.NewReal()
	start := wall.Now()

	fired := make(chan time.Time, 1)
	wall.After(5*time.Millisecond, func() { fired <- wall.Now() })

	select {
	case at := <-fired:
		if at.Before(start) {
			t.Fatalf("the timer fired at %v, before it was armed at %v", at, start)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the real timer did not fire")
	}
}

func TestTheRealTimerReportsACancel(t *testing.T) {
	wall := virtualtime.NewReal()

	cancelled := false
	cancel := wall.After(time.Hour, func() { cancelled = true })

	if !cancel() {
		t.Error("cancelling a waiting real timer answered false")
	}
	if cancel() {
		t.Error("cancelling a cancelled real timer answered true")
	}
	if cancelled {
		t.Error("a cancelled real timer fired")
	}
}

func TestTheRealRepeatingTimerRepeats(t *testing.T) {
	wall := virtualtime.NewReal()

	ticks := make(chan struct{}, 3)
	cancel := wall.Every(time.Millisecond, func() {
		select {
		case ticks <- struct{}{}:
		default:
		}
	})

	for i := 0; i < 3; i++ {
		select {
		case <-ticks:
		case <-time.After(5 * time.Second):
			t.Fatalf("the real repeating timer fired %d times in five seconds", i)
		}
	}

	cancel()

	if cancel() {
		t.Error("cancelling a cancelled real timer answered true")
	}
}

// The same contract as the clock's: a repeating timer needs somewhere to start
// from, and the wall clock says so the same way.
func TestTheRealRepeatingTimerNeedsAPositiveInterval(t *testing.T) {
	wall := virtualtime.NewReal()

	mustPanic(t, func() { wall.Every(0, func() {}) })
}
