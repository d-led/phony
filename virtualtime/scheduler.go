package virtualtime

import (
	"container/heap"
	"time"

	"github.com/d-led/phony"
)

// Timer is what an actor needs in order to be testable: somewhere to ask for
// "call this later". An actor that takes a Timer instead of reaching for the
// time package is testable in virtual time and runs on real time unchanged.
//
// A Timer cannot move time. The wall clock moves by itself, and a Clock is moved by
// whoever owns it: the test says how much time has passed, with Advance, AdvanceTo
// or Run, and the actor that was given the clock never does. An actor that could
// move its own test's clock would be testing itself.
type Timer interface {
	// After calls f once a duration has passed, and answers a cancel function
	// that reports whether the notification was still waiting.
	After(d time.Duration, f func()) func() bool

	// Every calls f after a duration has passed and every interval after that,
	// until it is cancelled. A non-positive interval panics.
	Every(interval time.Duration, f func()) func() bool

	// Now answers the time the Timer is keeping.
	Now() time.Time
}

// Clock is a Timer whose time belongs to a test, and the scheduler that runs the
// actors of that test. Create it with New, and call Reset when the test is done.
//
// A Clock is not safe for concurrent use: the point of it is that one thread
// does everything.
type Clock struct {
	now       time.Time
	ready     []func()
	timers    timerHeap
	seq       uint64
	previous  phony.Starter // the Starter that was installed before this clock
	installed bool
}

var _ Timer = (*Clock)(nil)

// New answers a clock and installs it in the package: from now on, an Inbox that
// receives its first message is drained by this clock's Run or Advance, not by a
// goroutine of its own.
//
// The installation is process-wide, because phony has one Starter for all
// Inboxes: keep the tests that use a clock serial, or install one in TestMain.
// The clock starts at the Unix epoch; only Advance moves it.
func New() *Clock {
	clock := &Clock{now: time.Unix(0, 0)}

	clock.previous = phony.SwapStarter(clock.started)
	clock.installed = true

	return clock
}

// Reset puts back the Starter that was installed when this clock was created,
// and forgets everything the clock was holding.
func (c *Clock) Reset() {
	if c.installed {
		phony.SwapStarter(c.previous)
		c.installed = false
	}

	c.ready = nil
	c.timers = nil
}

// Now answers the virtual time of the clock.
func (c *Clock) Now() time.Time {
	return c.now
}

// After calls f once the clock has advanced by d.
func (c *Clock) After(d time.Duration, f func()) func() bool {
	return c.at(c.now.Add(d), 0, f)
}

// WaitFor lets up to d of the clock's time pass, timer by timer, until f answers
// true, and answers whether it did. It is the clock owner's convenience for a test
// that does not know how much time is enough: it moves the clock the way Advance
// does, and stops as soon as f is satisfied, so it never moves past what was waited
// for.
func (c *Clock) WaitFor(d time.Duration, f func() bool) bool {
	deadline := c.now.Add(d)

	for !f() && c.now.Before(deadline) {
		c.Run()

		next := c.peekTimer()
		if next == nil || next.due.After(deadline) {
			c.AdvanceTo(deadline)
			break
		}

		c.AdvanceTo(next.due)
	}

	return f()
}

// Every calls f once the clock has advanced by interval, and every interval
// after that, until it is cancelled.
func (c *Clock) Every(interval time.Duration, f func()) func() bool {
	if interval <= 0 {
		panic("virtualtime: Every needs a positive interval")
	}

	return c.at(c.now.Add(interval), interval, f)
}

// Pending answers how many timers are waiting, and how many actor work items are
// ready to run. A test that expects the actors to be finished asserts on the
// second number instead of waiting for them to be.
func (c *Clock) Pending() (timers int, ready int) {
	waiting := 0

	for _, t := range c.timers {
		if !t.cancelled {
			waiting++
		}
	}

	return waiting, len(c.ready)
}

// Run processes everything that is ready now: the messages of the actors, and
// every message those messages send, until nothing is left to do. It does not
// move the clock, so it is how a test lets the actors catch up without making
// time pass.
func (c *Clock) Run() {
	for c.step() {
	}
}

// Advance moves the clock forward by d, running everything that becomes due on
// the way. A timer due on the instant the clock reaches has fired when the call
// returns.
func (c *Clock) Advance(d time.Duration) {
	c.AdvanceTo(c.now.Add(d))
}

// AdvanceTo is Advance to an instant instead of by a duration.
func (c *Clock) AdvanceTo(instant time.Time) {
	if instant.Before(c.now) {
		panic("virtualtime: time does not go backwards")
	}

	c.Run()

	for {
		next := c.peekTimer()
		if next == nil || next.due.After(instant) {
			break
		}

		// Take it out before firing: a repeating timer moves to its next instant,
		// and a timer that is not in the queue cannot break the queue's order.
		heap.Pop(&c.timers)
		c.now = next.due
		next.fire()

		if next.interval > 0 && !next.cancelled {
			heap.Push(&c.timers, next)
		}

		c.Run()
	}

	c.now = instant
	c.Run()
}

func (c *Clock) at(due time.Time, interval time.Duration, f func()) func() bool {
	if f == nil {
		panic("virtualtime: a timer needs something to call")
	}

	t := &timer{due: due, interval: interval, f: f, seq: c.seq}
	c.seq++
	heap.Push(&c.timers, t)

	return t.cancel
}

func (c *Clock) started(work func()) {
	c.ready = append(c.ready, work)
}

// step runs one actor's work. It answers false when nothing is ready, which
// means every actor is idle.
func (c *Clock) step() bool {
	if len(c.ready) == 0 {
		return false
	}

	work := c.ready[0]
	c.ready = c.ready[1:]
	work()

	return true
}

// peekTimer answers the timer that is due first, or nil. Cancelled timers that
// reached the top of the queue are dropped on the way.
func (c *Clock) peekTimer() *timer {
	for c.timers.Len() > 0 {
		t := c.timers[0]
		if !t.cancelled {
			return t
		}

		heap.Pop(&c.timers)
	}

	return nil
}

// timer is one pending notification. A repeating timer moves to its next instant
// when it fires; a one-shot is cancelled by firing.
type timer struct {
	due       time.Time
	interval  time.Duration
	f         func()
	seq       uint64
	cancelled bool
}

func (t *timer) fire() {
	if t.cancelled {
		return
	}

	f := t.f

	if t.interval > 0 {
		t.due = t.due.Add(t.interval)
	} else {
		t.cancelled = true
	}

	f()
}

func (t *timer) cancel() bool {
	if t.cancelled {
		return false
	}

	t.cancelled = true

	return true
}

// timerHeap orders timers by the instant they are due, and by the order they
// were scheduled when several are due at the same instant.
type timerHeap []*timer

func (h timerHeap) Len() int { return len(h) }

func (h timerHeap) Less(i, j int) bool {
	if !h[i].due.Equal(h[j].due) {
		return h[i].due.Before(h[j].due)
	}

	return h[i].seq < h[j].seq
}

func (h timerHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }

func (h *timerHeap) Push(x interface{}) { *h = append(*h, x.(*timer)) }

func (h *timerHeap) Pop() interface{} {
	old := *h
	last := old[len(old)-1]
	*h = old[:len(old)-1]

	return last
}
