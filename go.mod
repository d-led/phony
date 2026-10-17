module github.com/d-led/phony

go 1.19

require go.uber.org/goleak v1.2.1

// v0.0.3 handed the actor the clock's own business: its Timer carried WaitFor, so
// an actor could move the clock of the test it was running in. v0.0.4 takes that
// back off the interface — a Timer schedules and nothing more — so v0.0.3 is
// retracted rather than left to be picked up.
retract v0.0.3
