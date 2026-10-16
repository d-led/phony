// Package phony is a small actor model library for Go, inspired by the causal messaging system in the Pony programming language.
// An Actor is an interface satisfied by a lightweight Inbox struct.
// Structs that embed an Inbox satisfy an interface that allows them to send messages to eachother.
// Messages are functions of 0 arguments, typically closures, and should not perform blocking operations.
// Message passing is asynchronous, causal, and fast.
// Actors implemented by the provided Inbox struct are scheduled to prevent messages queues from growing too large, by pausing at safe breakpoints when an Actor detects that it sent something to another Actor whose inbox is flooded.
//
// This is the d-led fork of github.com/Arceliar/phony, and it adds one thing: the
// virtualtime package, in which an actor's timers are a value the test advances,
// so a test of an actor that waits for time does not wait. An actor that takes its
// timers from a virtualtime.Timer runs unchanged on real time and in such a test;
// examples/virtualtime-ticking is the same program run both ways.
package phony
