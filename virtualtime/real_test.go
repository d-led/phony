package virtualtime_test

import (
	"testing"
	"time"

	"github.com/Arceliar/phony/virtualtime"
)

// An actor written against a Timer does not know which one it was given, so the
// wall-clock Timer is tested to the same contract as the clock: it calls back
// when it is due, it reports a cancel, and it answers the time it is keeping.
func TestTheRealTimerCallsBack(t *testing.T) {
	real := virtualtime.NewReal()
	start := real.Now()

	fired := make(chan time.Time, 1)
	real.After(5*time.Millisecond, func() { fired <- real.Now() })

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
	real := virtualtime.NewReal()

	cancelled := false
	cancel := real.After(time.Hour, func() { cancelled = true })

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
	real := virtualtime.NewReal()

	ticks := make(chan struct{}, 3)
	cancel := real.Every(time.Millisecond, func() {
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
