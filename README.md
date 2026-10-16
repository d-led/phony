# Phony

[![CI](https://github.com/d-led/phony/actions/workflows/ci.yml/badge.svg)](https://github.com/d-led/phony/actions/workflows/ci.yml)
[![GoDoc](https://pkg.go.dev/badge/github.com/d-led/phony.svg)](https://pkg.go.dev/github.com/d-led/phony)

Phony is a [Pony](https://ponylang.io/)-inspired proof-of-concept implementation of shared-memory actor-model concurrency in the Go programming language. `Actor`s automatically manage goroutines and use asynchronous causal messaging (with backpressure) for communcation. This makes it easy to write programs that are free from deadlocks, goroutine leaks, and many of the `for` loops over `select` statements that show up in boilerplate code. The down side is that the code needs to be written in an asynchronous style, which is not idiomatic to Go, so it can take some getting used to.

Phony is [Arceliar](https://github.com/Arceliar/phony)'s library. This is the fork of
[d-led](https://github.com/d-led/phony): the library is the same, and the fork adds the testability
of time in the [`virtualtime`](virtualtime) package. The import path is `github.com/d-led/phony`.

## New: Fast Testability in Virtual Time

An actor that waits is what makes its test slow, and a test can do nothing about it: the time has to
pass. Give the actor its timers instead of letting it reach for the `time` package, and the same test
body runs on the wall clock and on a clock the test owns, where it says that the time has passed and
does not wait. `timeIt` below measures what the body really cost:

```go
timeIt(func() {
	ticker := NewTicker(RealTime())                 // production: the wall clock
	ticker.TickTimesAsync(3)
	ticker.WaitForTicksAtLeast(3, 10*time.Second)   // this waits for the ticks
	// a test asserts here: ticker.GetTicks() == 3
})

timeIt(func() {
	clock := VirtualTime()                          // the same actor on a clock
	defer clock.Reset()                             // this test owns

	ticker := NewTicker(clock)
	ticker.TickTimesAsync(3)
	ticker.WaitForTicksAtLeast(3, 10*time.Second)   // the same ticks, and the
	// ticker.GetTicks() == 3, in no time at all    // three seconds cost nothing
})
```

```
real time:    3 ticks in 3.001320167s
virtual time: 3 ticks in 28.5µs
```

That is [`examples/virtualtime-ticking`](examples/virtualtime-ticking): the actor in
[main.go](examples/virtualtime-ticking/main.go) and the same runs as tests in
[main_test.go](examples/virtualtime-ticking/main_test.go). The clock and its tests are in
[`virtualtime/`](virtualtime), and `go doc github.com/d-led/phony/virtualtime` is the reference. The
benchmarks and the implementation notes of the original library are in
[Arceliar's README](https://github.com/Arceliar/phony#readme).
