// Package virtualtime is a clock for tests: the test says how much time has
// passed, and the actors of the test do their work on the test's thread while it
// does.
//
// An actor that waits for time is what makes its test slow, and a test can do
// nothing about it — the time has to pass. Give the actor its timers instead (a
// Timer), and the same test body runs on the wall clock and on a clock the test
// owns, where it says the time has passed and does not wait:
//
//	timeIt(func() int { // on the wall clock
//		ticker := NewTicker(RealTime())
//		ticker.TickTimesAsync(3)
//		ticker.WaitForTicksAtLeast(3, 10*time.Second)
//
//		return ticker.GetTicks()
//	})
//
//	timeIt(func() int { // on a clock the test owns
//		clock := VirtualTime()
//		defer clock.Reset()
//
//		ticker := NewTicker(clock)
//		ticker.TickTimesAsync(3)
//		ticker.WaitForTicksAtLeast(3, 10*time.Second)
//
//		return ticker.GetTicks()
//	})
//
// Waiting is the one thing the two clocks do differently: on the wall clock it is
// waiting, and on a Clock it is the step over the time that would have passed. The
// rest follows from the clock running an Inbox's work itself instead of a
// goroutine of its own: nothing happens behind the test's back, the timers fire in
// the order they are due, and the same test run twice does the same thing twice.
//
// Example is the package's smallest actor: an alarm that rings in a hundred days,
// and a test that says the hundred days have passed. examples/virtualtime-ticking
// runs the comparison above as a program.
package virtualtime
