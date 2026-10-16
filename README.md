# Phony

[![CI](https://github.com/d-led/phony/actions/workflows/ci.yml/badge.svg)](https://github.com/d-led/phony/actions/workflows/ci.yml)
[![GoDoc](https://pkg.go.dev/badge/github.com/d-led/phony.svg)](https://pkg.go.dev/github.com/d-led/phony)

Phony is a [Pony](https://ponylang.io/)-inspired proof-of-concept implementation of shared-memory actor-model concurrency in the Go programming language. `Actor`s automatically manage goroutines and use asynchronous causal messaging (with backpressure) for communcation. This makes it easy to write programs that are free from deadlocks, goroutine leaks, and many of the `for` loops over `select` statements that show up in boilerplate code. The down side is that the code needs to be written in an asynchronous style, which is not idiomatic to Go, so it can take some getting used to.

Phony is [Arceliar](https://github.com/Arceliar/phony)'s library, and this is a fork by
[d-led](https://github.com/d-led), imported as `github.com/d-led/phony`. The library is unchanged; the
fork adds the [`virtualtime`](virtualtime) package, which puts the time an actor waits in under the
control of the test that waits. Arceliar's own README is kept verbatim in
[README.upstream.md](README.upstream.md), and Phony stays under the MPL-2.0, as the original: see
[LICENSE](LICENSE).

## New: Fast Testability in Virtual Time

An actor that waits for time decides how long its test takes: three ticks a second apart cost the test
three seconds, and no assertion can make that shorter. An actor that takes its timers from a
`virtualtime.Timer` instead of reaching for the `time` package runs unchanged in production and, in a
test, on a clock the test owns, where the test says that the time has passed. The two bodies below are
[`examples/virtualtime-ticking`](examples/virtualtime-ticking), and `timeIt` reports what each cost:

```go
timeIt(func() int {                                 // the wall clock: this waits
	ticker := NewTicker(RealTime())
	ticker.TickTimesAsync(3)
	ticker.WaitForTicksAtLeast(3, 10*time.Second)

	return ticker.GetTicks()
})

timeIt(func() int {                                 // a clock the program owns:
	clock := VirtualTime()                          // the same ticks, and no
	defer clock.Reset()                             // time at all

	ticker := NewTicker(clock)
	ticker.TickTimesAsync(3)
	ticker.WaitForTicksAtLeast(3, 10*time.Second)

	return ticker.GetTicks()
})
```

```shell
real time:    3 ticks in 3.001552708s
virtual time: 3 ticks in 53.834µs
```

The first line is the three seconds the ticker takes; the second is the cost of the run itself, since
there is nothing on that clock to wait for.

The ticker is [main.go](examples/virtualtime-ticking/main.go);
[main_test.go](examples/virtualtime-ticking/main_test.go) runs the same lines as tests on both clocks,
and [property_test.go](examples/virtualtime-ticking/property_test.go) drives the ticker with generated
schedules. An actor that waits once, and the test that decides the wait is over, is the package's
example in [`virtualtime/alarm_test.go`](virtualtime/alarm_test.go): an alarm that rings in a hundred
days —

```go
rang := alarm.RingAfter(100 * 24 * time.Hour)   // an actor, on the clock of the test
clock.Advance(100 * 24 * time.Hour)             // the test: the hundred days, now
```

[`virtualtime/`](virtualtime) holds the clock and its tests, and `go doc
github.com/d-led/phony/virtualtime` is the reference. Virtual time settles what the actors of a test do
to each other, and it is not a substitute for the wall clock: the ticker's own test still waits its
three seconds on one. The tests that hold the clock to this run on the race detector, check that the
actor systems leave no goroutines behind, and run under Go's memory checkers where the platform has
them; they are the only place that needs more than the standard library, `go.uber.org/goleak`.
