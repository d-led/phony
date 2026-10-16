package phony

// Starter runs the worker of an Inbox that was idle and has just received its
// first message. The worker drains the Inbox, and the Inbox goes back to being
// idle when it runs out of messages.
//
// The default starts a goroutine of its own, which is what an Inbox does on its
// own. A test can install a Starter of its own to run the work on a thread it
// controls: then nothing happens behind the test's back, the order of everything
// that happens is decided by the test, and a test can report what is still
// pending instead of waiting for it.
//
// It is a variable of the package rather than a field of an Inbox because an
// Inbox is meant to be embedded in user structs and its size is part of its
// design; a test that installs a Starter is expected to install it for the whole
// process (in TestMain) or to keep its tests serial.
var Starter = func(work func()) { go work() }
