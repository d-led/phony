// Package virtualtime is a clock for tests: time is a value the test advances,
// and the work of every actor under test happens on the thread that advances it.
//
// A phony Inbox has no clock and never does anything on a timer: time enters
// actor code only where the code asks for it. An actor that takes its timers from
// a Timer is therefore the same actor in a test and in production. This is the
// whole actor:
//
//	// Ticker ticks every interval until the duration it was asked for has passed.
//	type Ticker struct {
//		phony.Inbox
//		timer    virtualtime.Timer
//		interval time.Duration
//		ticks    int
//	}
//
//	func NewTicker(timer virtualtime.Timer, interval time.Duration) *Ticker {
//		return &Ticker{timer: timer, interval: interval}
//	}
//
//	// TickFor ticks every interval until d has passed.
//	func (t *Ticker) TickFor(d time.Duration) {
//		left := int(d / t.interval)
//
//		// Declared first, because the last tick cancels it.
//		var cancel func() bool
//		cancel = t.timer.Every(t.interval, func() {
//			t.Act(nil, func() {
//				if left <= 0 {
//					cancel()
//					return
//				}
//				left--
//				t.ticks++
//			})
//		})
//	}
//
//	// Ticks is the count, read inside the actor: on the wall clock the actor has a
//	// goroutine of its own, so the count is asked of it rather than read from it.
//	func (t *Ticker) Ticks() int { return t.ticks }
//
// On the wall clock, thirty ticks of a hundred milliseconds are three seconds of
// somebody's life, and a test can do nothing but wait for them, with a deadline.
// On a clock the test owns, the same thirty ticks and the same three seconds take
// no time at all, and the assertion is exact, because the test decided that the
// time had passed:
//
//	clock := virtualtime.New()
//	defer clock.Reset()
//
//	ticker := NewTicker(clock, 100*time.Millisecond)
//
//	clock.Block(ticker, func() { ticker.TickFor(3 * time.Second) })
//	clock.Advance(3 * time.Second)
//
//	ticks := 0
//	clock.Block(ticker, func() { ticks = ticker.Ticks() })
//
//	if ticks != 30 {
//		t.Fatalf("ticked %d times in three seconds, want 30", ticks)
//	}
//
// Example is that program, and examples/virtualtime-ticking is the same thing as
// a program one can run, printing what each of the two clocks costs.
//
// Real is the same Timer backed by wall-clock time, so the actor under test does
// not know which of the two it was given: NewReal answers it.
//
// Nothing waits and nothing races: the actors run on the test's thread because the
// clock installed itself as the package's phony.Starter (phony.SwapStarter), which
// hands an Inbox's work to the clock instead of to a goroutine; messages are
// processed in the order they were sent; and timers fire in the order they are due,
// in the order they were scheduled when several are due at the same instant. The
// same test run twice does the same thing twice.
package virtualtime
