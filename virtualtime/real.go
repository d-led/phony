package virtualtime

import (
	"sync"
	"time"
)

// Real is a Timer backed by wall-clock time, so an actor written against a
// Timer runs unchanged in production and in a test that advances a Clock.
type Real struct{}

var _ Timer = (*Real)(nil)

// NewReal answers the wall-clock Timer.
func NewReal() *Real {
	return &Real{}
}

// After calls f once d has passed. The cancel function reports whether the
// notification was still waiting.
func (r *Real) After(d time.Duration, f func()) func() bool {
	timer := time.AfterFunc(d, f)

	return timer.Stop
}

// Every calls f every interval until it is cancelled: the notifications do not
// overlap, and one may be in flight when a cancel returns. The cancel function
// reports whether it cancelled anything.
func (r *Real) Every(interval time.Duration, f func()) func() bool {
	if interval <= 0 {
		panic("virtualtime: Every needs a positive interval")
	}

	ticker := time.NewTicker(interval)
	stopped := make(chan struct{})
	once := sync.Once{}

	go func() {
		defer ticker.Stop()

		for {
			select {
			case <-stopped:
				return
			case <-ticker.C:
				f()
			}
		}
	}()

	return func() bool {
		cancelled := false

		once.Do(func() {
			close(stopped)
			cancelled = true
		})

		return cancelled
	}
}

// Now answers the wall clock.
func (r *Real) Now() time.Time {
	return time.Now()
}

// WaitFor waits up to d of wall-clock time for f to answer true, and answers
// whether it did. This is the wall clock's half of the trade the Clock makes: a
// test that waits with one waits with the other, and only the clock changes.
func (r *Real) WaitFor(d time.Duration, f func() bool) bool {
	deadline := time.Now().Add(d)

	for !f() {
		if time.Now().After(deadline) {
			return false
		}

		time.Sleep(time.Millisecond)
	}

	return true
}
