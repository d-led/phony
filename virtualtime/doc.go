// Package virtualtime is a clock for tests: time is a value the test advances, and
// the actors under test do their work on the test's thread while it does.
//
// An actor that waits is what makes its test slow: a thirty-second timeout, a
// retry in a minute, an alarm in a day. Give an actor its timers instead, and it is
// the same actor in a test and in production. This is the whole actor:
//
//	// Ticker ticks every interval until the duration it was asked for has passed.
//	type Ticker struct {
//		phony.Inbox
//		timer    virtualtime.Timer
//		interval time.Duration
//		ticks    int
//	}
//
//	// TickFor asks the ticker to tick every interval until d has passed, and
//	// answers the channel the number of ticks arrives on.
//	func (t *Ticker) TickFor(d time.Duration) <-chan int {
//		ticks := make(chan int, 1)
//
//		t.Act(nil, func() {
//			left := int(d / t.interval)
//
//			// Declared first, because the last tick cancels it.
//			var cancel func() bool
//			cancel = t.timer.Every(t.interval, func() {
//				t.Act(nil, func() {
//					// The ticker of the wall clock may have one tick in flight
//					// after the last one was counted.
//					if left <= 0 {
//						cancel()
//						return
//					}
//
//					left--
//					t.ticks++
//
//					if left == 0 {
//						cancel()
//						ticks <- t.ticks
//					}
//				})
//			})
//		})
//
//		return ticks
//	}
//
// and this is the whole test of it: three seconds that have passed by the time the
// clock reaches the instant, with nothing to wait for and nothing to poll:
//
//	clock := virtualtime.New()
//	defer clock.Reset()
//
//	ticker := NewTicker(clock, 100*time.Millisecond)
//
//	ticks := ticker.TickFor(3 * time.Second)
//	clock.Advance(3 * time.Second)
//
//	if count := <-ticks; count != 30 {
//		t.Fatalf("ticked %d times in three seconds, want 30", count)
//	}
//
// In production the same actor is given NewReal() and nothing about it changes;
// only then do the three seconds take three seconds. Example is the test above, and
// examples/virtualtime-ticking is the same comparison as a program one can run.
//
// What a clock gives a test:
//
//   - everything due up to the instant Advance reaches has happened when it
//     returns, so there is nothing to wait for and the assertions are exact;
//   - the work happens on the test's thread, because the clock installed itself as
//     the package's phony.Starter (phony.SwapStarter), which hands an Inbox's work
//     to the clock instead of to a goroutine: no actor of the test runs while the
//     test is not looking, and nothing races;
//   - messages are processed in the order they were sent, and timers fire in the
//     order they are due, in the order they were scheduled when several are due at
//     the same instant;
//   - the same test run twice does the same thing twice.
package virtualtime
