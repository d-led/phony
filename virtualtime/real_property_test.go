package virtualtime_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/d-led/phony/virtualtime"
)

// The wall clock with the properties of the clock a test owns, at the only speed
// the wall clock has. A firing reaches a callback through a goroutine of its own,
// so the wall clock can only keep an order the machine can wake a goroutine inside:
// firings 25ms apart came back inverted on a loaded machine, and the schedule here
// keeps them 100ms apart, which a busy machine can keep and the clock cannot fake.
// Four cases, then, so that a property still costs about a second.

const (
	scheduleOf4  = 4
	realStep     = 100 * time.Millisecond
	realCaseRuns = 4
)

// realDelaysOf answers one delay per generated value: 100ms, 200ms, 300ms and
// 400ms, distinct, in the order the values pick.
func realDelaysOf(raw [scheduleOf4]int8) []time.Duration {
	taken := make([]bool, scheduleOf4)
	delays := make([]time.Duration, 0, scheduleOf4)

	for _, value := range raw {
		step := oneTo(value, scheduleOf4) - 1
		for taken[step] {
			step = (step + 1) % scheduleOf4
		}

		taken[step] = true
		delays = append(delays, time.Duration(step+1)*realStep)
	}

	return delays
}

// realTrace arms one timer per delay on the wall clock and answers the order they
// fire in. A -1 at the end means the wall clock did not fire when it said it would.
func realTrace(delays []time.Duration) []int {
	wall := virtualtime.NewReal()

	fired := make(chan int, len(delays))
	for i, delay := range delays {
		i, delay := i, delay
		wall.After(delay, func() { fired <- i })
	}

	order := make([]int, 0, len(delays))
	for range delays {
		select {
		case i := <-fired:
			order = append(order, i)
		case <-time.After(time.Second):
			return append(order, -1)
		}
	}

	return order
}

// clockTrace answers the same, for a clock the test owns, which costs nothing.
func clockTrace(delays []time.Duration) []int {
	order := make([]int, 0, len(delays))
	for _, fired := range traceOf(delays) {
		order = append(order, fired.timer)
	}

	return order
}

func TestTheWallClockFiresInTheOrderItWasAskedFor(t *testing.T) {
	property := func(raw [scheduleOf4]int8) bool {
		delays := realDelaysOf(raw)

		return reflect.DeepEqual(realTrace(delays), dueOrder(delays))
	}

	check(t, property, realCaseRuns)
}

// The property that says the two clocks are the same clock: the same schedule
// fires in the same order on the wall clock and on a clock the test advances, and
// only the time it takes to get there is different.
func TestTheTwoClocksFireInTheSameOrder(t *testing.T) {
	property := func(raw [scheduleOf4]int8) bool {
		delays := realDelaysOf(raw)

		return reflect.DeepEqual(realTrace(delays), clockTrace(delays))
	}

	check(t, property, realCaseRuns)
}

// A timer of the wall clock is never early: it fires when it is due, or later.
func TestTheWallClockIsNeverEarly(t *testing.T) {
	property := func(raw [scheduleOf4]int8) bool {
		delays := realDelaysOf(raw)

		wall := virtualtime.NewReal()
		start := time.Now()
		early := make(chan bool, len(delays))

		for _, delay := range delays {
			delay := delay
			wall.After(delay, func() { early <- time.Since(start) < delay })
		}

		for range delays {
			select {
			case wasEarly := <-early:
				if wasEarly {
					return false
				}
			case <-time.After(time.Second):
				return false
			}
		}

		return true
	}

	check(t, property, realCaseRuns)
}
