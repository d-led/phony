package virtualtime_test

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/d-led/phony"
	"github.com/d-led/phony/virtualtime"
)

// The contract of the clock itself, without actors in the way.
func TestATimerDoesNotFireBeforeItsInstant(t *testing.T) {
	clock := virtualtime.New()
	defer clock.Reset()

	fired := 0
	clock.After(time.Second, func() { fired++ })

	clock.Advance(999 * time.Millisecond)
	if fired != 0 {
		t.Fatalf("fired %d times before its instant", fired)
	}

	clock.Advance(time.Millisecond)
	if fired != 1 {
		t.Fatalf("fired %d times on its instant, want 1", fired)
	}
}

func TestARepeatingTimerFiresOncePerInterval(t *testing.T) {
	clock := virtualtime.New()
	defer clock.Reset()

	fired := 0
	clock.Every(100*time.Millisecond, func() { fired++ })

	clock.Advance(250 * time.Millisecond)

	if fired != 2 {
		t.Fatalf("fired %d times in 250ms of a 100ms interval, want 2", fired)
	}
}

func TestCancellingAnswersWhetherTheTimerWasStillWaiting(t *testing.T) {
	clock := virtualtime.New()
	defer clock.Reset()

	fired := 0
	cancel := clock.After(time.Second, func() { fired++ })

	if !cancel() {
		t.Error("cancelling a waiting timer answered false")
	}
	clock.Advance(10 * time.Second)
	if fired != 0 {
		t.Fatalf("a cancelled timer fired %d times", fired)
	}
	if cancel() {
		t.Error("cancelling a cancelled timer answered true")
	}

	fired = 0
	afterFiring := clock.After(time.Second, func() { fired++ })
	clock.Advance(time.Second)
	if fired != 1 {
		t.Fatalf("fired %d times, want 1", fired)
	}
	if afterFiring() {
		t.Error("cancelling a timer that has fired answered true")
	}
}

func TestCancellingARepeatingTimerStopsIt(t *testing.T) {
	clock := virtualtime.New()
	defer clock.Reset()

	fired := 0
	cancel := clock.Every(time.Second, func() { fired++ })

	clock.Advance(3 * time.Second)
	cancel()
	clock.Advance(10 * time.Second)

	if fired != 3 {
		t.Fatalf("fired %d times, want the 3 before the cancel", fired)
	}
}

func TestTimersOnTheSameInstantFireInTheOrderTheyWereScheduled(t *testing.T) {
	clock := virtualtime.New()
	defer clock.Reset()

	var fired []string
	for _, name := range []string{"first", "second", "third"} {
		name := name
		clock.After(time.Second, func() { fired = append(fired, name) })
	}

	clock.Advance(time.Second)

	if got := strings.Join(fired, ","); got != "first,second,third" {
		t.Fatalf("same-instant timers fired as %q", got)
	}
}

func TestATimerScheduledWhileTheClockRunsIsMeasuredFromItsOwnInstant(t *testing.T) {
	clock := virtualtime.New()
	defer clock.Reset()

	var fired []string
	clock.After(time.Second, func() {
		fired = append(fired, "outer")
		clock.After(100*time.Millisecond, func() { fired = append(fired, "inner") })
	})

	clock.Advance(time.Second)
	if got := strings.Join(fired, ","); got != "outer" {
		t.Fatalf("on the outer timer's instant: %q", got)
	}

	clock.Advance(99 * time.Millisecond)
	if got := strings.Join(fired, ","); got != "outer" {
		t.Fatalf("99ms after the inner timer was scheduled: %q", got)
	}

	clock.Advance(time.Millisecond)
	if got := strings.Join(fired, ","); got != "outer,inner" {
		t.Fatalf("on the inner timer's instant: %q", got)
	}
}

func TestTimeDoesNotGoBackwards(t *testing.T) {
	clock := virtualtime.New()
	defer clock.Reset()

	clock.Advance(time.Second)

	mustPanic(t, func() { clock.Advance(-time.Millisecond) })
	mustPanic(t, func() { clock.AdvanceTo(clock.Now().Add(-time.Second)) })
}

func TestARepeatingTimerNeedsAPositiveInterval(t *testing.T) {
	clock := virtualtime.New()
	defer clock.Reset()

	mustPanic(t, func() { clock.Every(0, func() {}) })
}

func TestATimerNeedsSomethingToCall(t *testing.T) {
	clock := virtualtime.New()
	defer clock.Reset()

	mustPanic(t, func() { clock.After(time.Second, nil) })
}

func TestPendingAnswersWhatIsStillToCome(t *testing.T) {
	clock := virtualtime.New()
	defer clock.Reset()

	clock.After(time.Second, func() {})
	clock.Every(time.Minute, func() {})

	if timers, ready := clock.Pending(); timers != 2 || ready != 0 {
		t.Fatalf("pending: %d timers, %d ready; want 2 and 0", timers, ready)
	}

	clock.Advance(time.Second)

	if timers, _ := clock.Pending(); timers != 1 {
		t.Fatalf("pending timers after the one-shot fired: %d, want 1", timers)
	}
}

// The scenario of the package's own documentation, with an actor of each kind on
// one clock: the run is the same every time, which is what installing the
// Starter seam buys over the default goroutine per inbox.
func TestTheSameScenarioRunsTheSameWayTwice(t *testing.T) {
	want := []string{"tick at 1h0m0s", "alarm at 2h0m0s", "tick at 2h0m0s"}

	first := runScenario(t)
	second := runScenario(t)

	if !reflect.DeepEqual(first, second) {
		t.Fatalf("the same scenario ran differently:\n%v\n%v", first, second)
	}
	if !reflect.DeepEqual(first, want) {
		t.Fatalf("the scenario ran as %v, want %v", first, want)
	}
}

func runScenario(t *testing.T) []string {
	t.Helper()

	clock := virtualtime.New()
	defer clock.Reset()

	start := clock.Now()
	var trace []string
	note := func(what string) {
		trace = append(trace, fmt.Sprintf("%s at %v", what, clock.Now().Sub(start)))
	}

	alarm := &alarm{timer: clock, note: note}
	ticker := &ticker{timer: clock, note: note}

	clock.Block(alarm, func() { alarm.RingAfter(2 * time.Hour) })
	clock.Block(ticker, func() { ticker.TickEvery(time.Hour) })

	clock.Advance(2 * time.Hour)

	return trace
}

type alarm struct {
	phony.Inbox
	timer virtualtime.Timer
	note  func(string)
}

func (a *alarm) RingAfter(d time.Duration) {
	a.timer.After(d, func() {
		a.Act(nil, func() { a.note("alarm") })
	})
}

type ticker struct {
	phony.Inbox
	timer virtualtime.Timer
	note  func(string)
}

func (t *ticker) TickEvery(interval time.Duration) {
	t.timer.Every(interval, func() {
		t.Act(nil, func() { t.note("tick") })
	})
}

func mustPanic(t *testing.T, f func()) {
	t.Helper()

	defer func() {
		if recover() == nil {
			t.Fatal("expected a panic")
		}
	}()

	f()
}
