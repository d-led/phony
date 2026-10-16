// Package virtualtime is a clock for tests: the test says how much time has
// passed, and the actors of the test do their work on the test's thread while it
// does.
//
// An actor that waits for time is what makes its test slow, and a test can do
// nothing about it — the time has to pass. Give the actor its timers instead (a
// Timer), and the time becomes the test's business: it waits for the wall clock, and
// for the clock it owns it says how much time has passed. The actor is given either
// and cannot tell them apart, and it has no way to move time at all:
//
//	wall := NewReal() // the wall clock, and a test that waits on it
//	ticker := NewTicker(wall)
//	ticker.TickTimesAsync(3)
//	wall.WaitFor(10*time.Second, func() bool { return ticker.GetTicks() >= 3 })
//
//	clock := New() // a clock the test owns, and the same ticks
//	defer clock.Reset()
//
//	ticker := NewTicker(clock)
//	ticker.TickTimesAsync(3)
//	clock.Advance(3 * tickEvery) // the test says the three seconds have passed
//
// Advance moves the clock to the instant the test asks for, timer by timer, and so
// does AdvanceTo; Run lets the actors catch up without moving it at all, and WaitFor
// is Advance up to the instant a condition first holds. What follows from the clock
// running an Inbox's work itself instead of a goroutine of its own: nothing happens
// behind the test's back, the timers fire in the order they are due, and the same
// test run twice does the same thing twice.
//
// Example is the package's smallest actor: an alarm that rings in a hundred days,
// and a test that says the hundred days have passed. examples/virtualtime-ticking
// runs the comparison above as a program.
package virtualtime
