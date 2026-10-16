package virtualtime_test

import (
	"reflect"
	"sort"
	"testing"
	"testing/quick"
	"time"

	"github.com/d-led/phony/virtualtime"
)

// The properties of the clock: what has to hold for every schedule, and not only
// for the schedules the other tests write out by hand. The generated numbers are
// squashed into milliseconds, so that the properties are about the clock and not
// about the arithmetic of time.Duration.
//
// A schedule is a list of delays, with ties left in: two timers due at the same
// instant are part of what has to hold.

const (
	scheduleSize = 6
	longestDelay = 20 * time.Millisecond
)

// firing is one timer going off: which timer, and when the clock said it did.
type firing struct {
	timer int
	at    time.Duration
}

// delaysOf answers a schedule of delays: one per generated value, from 1ms to
// 20ms.
func delaysOf(raw [scheduleSize]int8) []time.Duration {
	delays := make([]time.Duration, 0, len(raw))

	for _, value := range raw {
		delays = append(delays, milliseconds(value%20)+time.Millisecond)
	}

	return delays
}

func milliseconds(value int8) time.Duration {
	if value < 0 {
		value = -value
	}

	return time.Duration(value) * time.Millisecond
}

// oneTo answers a number from 1 to upTo, from a generated value: a negative
// generator value is as ordinary as a positive one, and it must not turn into a
// negative expectation.
func oneTo(value int8, upTo int) int {
	return int(milliseconds(value%20)/time.Millisecond)%upTo + 1
}

// dueOrder answers the timers in the order they are due, and the ones due at the
// same instant in the order they were armed.
func dueOrder(delays []time.Duration) []int {
	order := make([]int, len(delays))
	for i := range order {
		order[i] = i
	}

	sort.SliceStable(order, func(a, b int) bool {
		return delays[order[a]] < delays[order[b]]
	})

	return order
}

// traceOf arms one timer per delay, in the order the delays are given, advances
// the clock past the whole schedule, and answers what fired and when.
func traceOf(delays []time.Duration) []firing {
	clock := virtualtime.New()
	defer clock.Reset()

	return traceOn(clock, delays, func(end time.Duration) { clock.Advance(end) })
}

// traceOn arms the timers on a clock of the caller's choosing, and lets the caller
// say how the time passes, which is the only difference between the two clocks.
func traceOn(clock *virtualtime.Clock, delays []time.Duration, passTime func(end time.Duration)) []firing {
	fired := make([]firing, 0, len(delays))
	start := clock.Now()

	for i, delay := range delays {
		i, delay := i, delay
		clock.After(delay, func() {
			fired = append(fired, firing{timer: i, at: clock.Now().Sub(start)})
		})
	}

	passTime(longestOf(delays))

	return fired
}

func longestOf(delays []time.Duration) time.Duration {
	longest := time.Duration(0)

	for _, d := range delays {
		if d > longest {
			longest = d
		}
	}

	return longest
}

// check runs a property many times, and a few times when it costs real time.
func check(t *testing.T, property interface{}, maxCount int) {
	t.Helper()

	if err := quick.Check(property, &quick.Config{MaxCount: maxCount}); err != nil {
		t.Error(err)
	}
}

// The one that matters most: a timer goes off when it is due and not before, and
// timers go off in the order they are due.
func TestEveryTimerFiresWhenItIsDue(t *testing.T) {
	property := func(raw [scheduleSize]int8) bool {
		delays := delaysOf(raw)
		fired := traceOf(delays)
		order := dueOrder(delays)

		if len(fired) != len(delays) {
			return false
		}

		for i, f := range fired {
			if f.timer != order[i] || f.at != delays[f.timer] {
				return false
			}
		}

		return true
	}

	check(t, property, 300)
}

// Advancing to an instant in one go and arriving at it in steps are the same
// thing, whichever steps the test picks.
func TestAdvancingInStepsIsTheSameAsAdvancingAtOnce(t *testing.T) {
	property := func(raw [scheduleSize]int8, split int8) bool {
		delays := delaysOf(raw)
		atOnce := traceOf(delays)

		clock := virtualtime.New()
		defer clock.Reset()

		inSteps := traceOn(clock, delays, func(end time.Duration) {
			first := time.Duration(oneTo(split, int(end/time.Millisecond)+1)-1) * time.Millisecond
			clock.Advance(first)
			clock.Advance(end - first)
		})

		return reflect.DeepEqual(atOnce, inSteps)
	}

	check(t, property, 200)
}

// The same schedule, run twice, does the same thing twice: which is what makes a
// failure in a test of an actor a fact and not a coincidence.
func TestTheSameScheduleRunsTheSameWayTwice(t *testing.T) {
	property := func(raw [scheduleSize]int8) bool {
		delays := delaysOf(raw)

		return reflect.DeepEqual(traceOf(delays), traceOf(delays))
	}

	check(t, property, 200)
}

// A repeating timer fires once per interval, and the number of firings is the
// whole number of intervals that fit in the time that passed.
func TestARepeatingTimerFiresAsOftenAsTheIntervalsFit(t *testing.T) {
	property := func(interval, span int8) bool {
		each := milliseconds(interval%10) + time.Millisecond
		until := milliseconds(span % 40)

		clock := virtualtime.New()
		defer clock.Reset()

		fired := 0
		cancel := clock.Every(each, func() { fired++ })

		clock.Advance(until)
		cancel()

		return fired == int(until/each)
	}

	check(t, property, 300)
}

// Waiting stops at the instant it was waiting for, and does not move the clock
// past it.
func TestWaitingStopsAtTheInstantItIsWaitingFor(t *testing.T) {
	property := func(raw [scheduleSize]int8, wanted int8) bool {
		delays := delaysOf(raw)
		last := oneTo(wanted, len(delays))

		clock := virtualtime.New()
		defer clock.Reset()

		start := clock.Now()
		fired := 0
		at := time.Duration(-1)

		for _, delay := range delays {
			clock.After(delay, func() {
				fired++
				if fired == last {
					at = clock.Now().Sub(start)
				}
			})
		}

		if !clock.WaitFor(longestOf(delays)+time.Minute, func() bool { return fired >= last }) {
			return false
		}

		return clock.Now().Sub(start) == at
	}

	check(t, property, 300)
}

// A wait that has nothing to wait for uses exactly the time it was given, and
// answers that it gave up.
func TestWaitingGivesUpWhenTheTimeItWasGivenIsUp(t *testing.T) {
	property := func(given int8) bool {
		clock := virtualtime.New()
		defer clock.Reset()

		start := clock.Now()
		enough := milliseconds(given % 20)
		never := func() bool { return false }

		if clock.WaitFor(enough, never) {
			return false
		}

		return clock.Now().Sub(start) == enough
	}

	check(t, property, 200)
}

// an odd generated value says: cancel this timer.
func wasCancelled(value int8) bool { return value%2 != 0 }

// cancelTheOddOnes takes out the timers the generated values point at and answers
// whether every one of those cancels found its timer still waiting.
func cancelTheOddOnes(cancels []func() bool, cancelled [scheduleSize]int8) bool {
	everyCancelFoundItsTimer := true

	for i, cancel := range cancels {
		if wasCancelled(cancelled[i]) && !cancel() {
			everyCancelFoundItsTimer = false
		}
	}

	return everyCancelFoundItsTimer
}

// theOnesLeftWaiting counts the timers no cancel was aimed at.
func theOnesLeftWaiting(cancelled [scheduleSize]int8) int {
	left := 0

	for _, value := range cancelled {
		if !wasCancelled(value) {
			left++
		}
	}

	return left
}

// theFiringsToExpect answers one count per timer: once, unless a cancel took it out.
func theFiringsToExpect(cancelled [scheduleSize]int8) []int {
	expected := make([]int, len(cancelled))

	for i, value := range cancelled {
		if !wasCancelled(value) {
			expected[i] = 1
		}
	}

	return expected
}

// firedAsExpected answers whether every timer fired the times the schedule asked for.
func firedAsExpected(fired, expected []int) bool {
	for i := range fired {
		if fired[i] != expected[i] {
			return false
		}
	}

	return true
}

// Cancelling takes out the timers it was asked to and leaves the others alone,
// and the timer that is still waiting is the one Pending counts.
func TestCancellingLeavesTheOtherTimersAlone(t *testing.T) {
	property := func(raw, cancelled [scheduleSize]int8) bool {
		delays := delaysOf(raw)

		clock := virtualtime.New()
		defer clock.Reset()

		fired := make([]int, len(delays))
		cancels := make([]func() bool, len(delays))

		for i, delay := range delays {
			fired[i] = 0
			at := i
			cancels[i] = clock.After(delay, func() { fired[at]++ })
		}

		if !cancelTheOddOnes(cancels, cancelled) {
			return false
		}

		timers, ready := clock.Pending()
		if timers != theOnesLeftWaiting(cancelled) || ready != 0 {
			return false
		}

		clock.Advance(longestOf(delays))

		return firedAsExpected(fired, theFiringsToExpect(cancelled))
	}

	check(t, property, 300)
}
