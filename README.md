# Phony

[![CI](https://github.com/d-led/phony/actions/workflows/ci.yml/badge.svg)](https://github.com/d-led/phony/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/d-led/phony)](https://goreportcard.com/report/github.com/d-led/phony)
[![GoDoc](https://pkg.go.dev/badge/github.com/d-led/phony.svg)](https://pkg.go.dev/github.com/d-led/phony)

Phony is a [Pony](https://ponylang.io/)-inspired implementation of the actor model in Go: `Actor`s
manage their own goroutines and pass messages asynchronously, causally and with backpressure — and
the parts of a concurrent system that wait for time are testable without waiting for it.

This is the `d-led` fork of [Arceliar/phony](https://github.com/Arceliar/phony), which is no longer
developed: the same library, with one seam and one package added for exactly that
(`phony.SwapStarter` and `virtualtime`). Nothing else changed, but the module path differs:

```go
import "github.com/d-led/phony"
```

## Actors that wait for time are testable in no time

A test of a concurrent system becomes slow, flaky or both as soon as time is involved: a
thirty-second timeout, a retry in a minute, an alarm in a day. Give the actor its timers instead of
letting it reach for the `time` package, and the test decides that the time has passed. This is the
whole actor, the same actor in a test and in production:

```go
// Ticker ticks every interval until the duration it was asked for has passed.
type Ticker struct {
	phony.Inbox
	timer    virtualtime.Timer
	interval time.Duration
	ticks    int
}

func NewTicker(timer virtualtime.Timer, interval time.Duration) *Ticker {
	return &Ticker{timer: timer, interval: interval}
}

// TickFor asks the ticker to tick every interval until d has passed, and answers
// the channel the number of ticks arrives on.
func (t *Ticker) TickFor(d time.Duration) <-chan int {
	ticks := make(chan int, 1)

	t.Act(nil, func() {
		left := int(d / t.interval)

		// Declared first, because the last tick cancels it.
		var cancel func() bool
		cancel = t.timer.Every(t.interval, func() {
			t.Act(nil, func() {
				// The ticker of the wall clock may have one tick in flight after
				// the last one was counted.
				if left <= 0 {
					cancel()
					return
				}

				left--
				t.ticks++

				if left == 0 {
					cancel()
					ticks <- t.ticks
				}
			})
		})
	})

	return ticks
}
```

And this is the whole test of it: three seconds that have passed by the time the clock reaches the
instant, with nothing to wait for and nothing to poll.

```go
func TestTickingTakesNoTime(t *testing.T) {
	clock := virtualtime.New()
	defer clock.Reset()

	ticker := NewTicker(clock, 100*time.Millisecond)

	ticks := ticker.TickFor(3 * time.Second)
	clock.Advance(3 * time.Second)

	if count := <-ticks; count != 30 {
		t.Fatalf("ticked %d times in three seconds, want 30", count)
	}
}
```

Nothing in the actor knows which clock it was given: in production the same ticker is
`NewTicker(virtualtime.NewReal(), 100*time.Millisecond)`, the same call is made, and *then* the
three seconds really do take three seconds.

## What it costs, and what it saves

Thirty ticks of a hundred milliseconds, the same actor and the same call, on the machine this was
written on:

```
wall clock:   30 ticks in 3.001056s
virtual time: 30 ticks in 11.834µs
the same 30 ticks, 3.001044166s sooner
```

The comparison is a program and not a mock-up:
[`examples/virtualtime-ticking`](examples/virtualtime-ticking) (`cd examples/virtualtime-ticking &&
go run .`). The wall-clock half of it is the test that can do nothing but wait, with a deadline and a
poll; the virtual half is the test above.

## Why the test can be exact instead of patient

- **Nothing waits.** A timer does not exist until it is due, and the work belongs to whoever asks
  the time: `Advance` runs everything that becomes due on the way, on the thread that called it.
  When `Advance` returns the actors are quiet, so the assertion right after it is exact — no poll, no
  deadline, no sleep.
- **Nothing happens behind the test's back.** `virtualtime.New()` installs the clock as the
  package's `phony.Starter` (`phony.SwapStarter`), which hands an Inbox's work to the clock instead
  of to a goroutine of its own: no actor of the test runs while the test is not looking, and nothing
  races.
- **The order is the order.** Messages are processed in the order they were sent, and timers fire in
  the order they are due, in the order they were scheduled when several are due at the same instant.
  The same test run twice does the same thing twice.
- **A hundred days, now.** `clock.Advance(100 * 24 * time.Hour)` is the line, and `TestWhyWait`
  takes it; `go test -race ./...` covers both timers.

The reference is `go doc github.com/d-led/phony/virtualtime`, and the tests of the package are the
contract of the clock: the instant a timer fires, repetition, cancellation, same-instant ordering,
and time that does not go backwards.

## Benchmarks

```
goos: linux
goarch: amd64
pkg: github.com/Arceliar/phony
cpu: Intel(R) Core(TM) i5-10300H CPU @ 2.50GHz
BenchmarkLoopActor-8                	38022962	        29.83 ns/op	       0 B/op	       0 allocs/op
BenchmarkLoopChannel-8              	29876192	        38.50 ns/op	       0 B/op	       0 allocs/op
BenchmarkSendActor-8                	14235270	        82.94 ns/op	       0 B/op	       0 allocs/op
BenchmarkSendChannel-8              	 8372472	       143.9 ns/op	       0 B/op	       0 allocs/op
BenchmarkRequestResponseActor-8     	10360731	       116.6 ns/op	       0 B/op	       0 allocs/op
BenchmarkRequestResponseChannel-8   	 4226506	       285.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkBlock-8                    	 2662929	       450.9 ns/op	      32 B/op	       2 allocs/op
PASS
ok  	github.com/Arceliar/phony	9.463s
```

These are the original author's measurements, taken before the fork, and they are left as they
were; the `pkg:` and `ok` lines name the module as it was then. These are microbenchmarks, but they
seem to indicate that `Actor` messaging and goroutine+channel operations have comparable cost. I
suspect that the difference is negligible in most applications.

## Implementation Details

The code base is short, under 100 source lines of code as of writing, so reading the code is probably the best way to see *what* it does, but that doesn't necessarily explain *why* certain design decisions were made. To elaborate on a few things:

- Phony only depends on packages from the standard library:
    - `runtime` for some scheduler manipulation (`Gosched()`).
    - `sync` for `sync.Pool`, to minimize allocations.
    - `sync/atomic` to implement the `Inbox`'s message queues.

- Attempts were make to make embedding and composition work:
    - `Actor` is an `interface` satisfied by the `Inbox` `struct`.
    - The zero value of an `Inbox` is a fully initialized and ready-to-use `Actor`
    - This means any `struct` that anonymously embeds an `Inbox` is an `Actor`
    - `struct`s that don't want to export the `Actor` interface can embed it as a field instead.

- `Inbox` was implemented with scalability in mind:
    - The `Inbox` is basically an atomically updated single-consumer multiple-producer linked list.
    - Pushing a message is wait-free -- no locks, spinlocks, or `CompareAndSwap` loops.
    - Popping messages is wait-free in the normal case, with a busy loop (`LoadPointer`) if popping the last message lost a race with a push.
    - When backpressure is required, it's implemented by sending two extra messages (one to the receiver of the original message, and one to the sender).

- The implementation aims to be as lightweight as reasonably possible:
    - On `x86_64`, an empty `Inbox` is 24 bytes, and messages overhead is 16 bytes, or half that on `x86`.
    - An `Actor` with an empty `Inbox` has no goroutine.
    - This means that idle `Actor`s can be collected as garbage when they're no longer reachable, just like any other `struct`.

