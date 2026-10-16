# Phony

[![CI](https://github.com/d-led/phony/actions/workflows/ci.yml/badge.svg)](https://github.com/d-led/phony/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/d-led/phony)](https://goreportcard.com/report/github.com/d-led/phony)
[![GoDoc](https://pkg.go.dev/badge/github.com/d-led/phony.svg)](https://pkg.go.dev/github.com/d-led/phony)

Phony is a [Pony](https://ponylang.io/)-inspired proof-of-concept implementation of shared-memory actor-model concurrency in the Go programming language. `Actor`s automatically manage goroutines and use asynchronous causal messaging (with backpressure) for communcation. This makes it easy to write programs that are free from deadlocks, goroutine leaks, and many of the `for` loops over `select` statements that show up in boilerplate code. The down side is that the code needs to be written in an asynchronous style, which is not idiomatic to Go, so it can take some getting used to.

This is the `d-led` fork of [Arceliar/phony](https://github.com/Arceliar/phony), which is no
longer developed. The fork is the same library, with one seam and one package added so that actors
can be tested without waiting for time (`phony.SwapStarter` and `virtualtime`, below); nothing else
changed. The module path differs, so the import is:

```go
import "github.com/d-led/phony"
```

## Testing actors that wait for time

An actor that takes its timers from a `virtualtime.Timer` instead of reaching for the `time`
package can be tested without waiting for time to pass. `virtualtime.Clock` is a `Timer` whose time
belongs to the test, and it installs itself as the package's `phony.Starter`, so the actors under
test run on the test's thread and nothing happens behind the test's back. This is the whole actor,
the same in both cases:

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

// TickFor ticks every interval until d has passed.
func (t *Ticker) TickFor(d time.Duration) {
	left := int(d / t.interval)

	// Declared first, because the last tick cancels it.
	var cancel func() bool
	cancel = t.timer.Every(t.interval, func() {
		t.Act(nil, func() {
			if left <= 0 {
				cancel()
				return
			}
			left--
			t.ticks++
		})
	})
}

// Ticks is the count, read inside the actor: on the wall clock the actor has a
// goroutine of its own, so the count is asked of it rather than read from it.
func (t *Ticker) Ticks() int { return t.ticks }
```

On the wall clock, the test waits for the ticks, because there is nothing else it can do: thirty
ticks of a hundred milliseconds are three seconds of somebody's life, and the assertion has to be a
poll with a deadline.

```go
func TestTickingOnTheWallClockTakesThreeSeconds(t *testing.T) {
	ticker := NewTicker(virtualtime.NewReal(), 100*time.Millisecond)

	start := time.Now()
	phony.Block(ticker, func() { ticker.TickFor(3 * time.Second) })

	// There is nothing to assert yet, so wait for the ticks with a deadline.
	deadline := time.Now().Add(6 * time.Second)
	var ticks int
	for {
		phony.Block(ticker, func() { ticks = ticker.Ticks() })
		if ticks >= 30 || time.Now().After(deadline) {
			break
		}
		time.Sleep(time.Millisecond)
	}

	if ticks != 30 {
		t.Fatalf("ticked %d times in three seconds, want 30", ticks)
	}
	t.Logf("wall clock: %d ticks in %v", ticks, time.Since(start))
}
```

On a clock the test owns, the same actor, the same thirty ticks and the same three seconds take no
time, and the assertion is exact: the ticks are already there because the test decided that they
happened.

```go
func TestTickingOnTheVirtualClockTakesNoTime(t *testing.T) {
	clock := virtualtime.New()
	defer clock.Reset()

	ticker := NewTicker(clock, 100*time.Millisecond)

	start := time.Now()
	clock.Block(ticker, func() { ticker.TickFor(3 * time.Second) })
	clock.Advance(3 * time.Second)

	ticks := 0
	clock.Block(ticker, func() { ticks = ticker.Ticks() })

	if ticks != 30 {
		t.Fatalf("ticked %d times in three seconds, want 30", ticks)
	}
	t.Logf("virtual time: %d ticks in %v", ticks, time.Since(start))
}
```

`go test ./virtualtime/ -run Ticking -v`, on the machine this was written on:

```
--- PASS: TestTickingOnTheWallClockTakesThreeSeconds (3.00s)
    ticker_test.go:83: wall clock: 30 ticks in 3.001190417s
--- PASS: TestTickingOnTheVirtualClockTakesNoTime (0.00s)
    ticker_test.go:102: virtual time: 30 ticks in 6.667µs
```

Both tests are in `virtualtime/ticker_test.go`, with the interval, the duration and the count out as
constants; the wall-clock one skips itself under `go test -short`. The package's `Example` is the
same actor, so `go doc ./virtualtime` leads with it.

The same thing as a program that depends on this module is in
[`examples/virtualtime-ticking`](examples/virtualtime-ticking): `cd examples/virtualtime-ticking &&
go run .` prints, on the same machine:

```
wall clock:   30 ticks in 3.001058792s
virtual time: 30 ticks in 34.417µs
the same 30 ticks, 3.001024375s sooner
```

In production the same actor is given `virtualtime.NewReal()` and nothing about it — or about its
timers, which do not exist until they are due — changes. `go doc ./virtualtime` describes the
contract; `go test ./virtualtime/` is the contract: the instant a timer fires, repetition,
cancellation, the order of timers due on the same instant, time that does not go backwards, and the
same scenario producing the same trace twice. `go test -race ./...` covers both timers.

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

