package main

import (
	"testing"

	"go.uber.org/goleak"
)

// The ticker of the example cancels its timer when its ticks are done, on either
// clock: this is what holds the example to leaving no goroutine behind, with the
// stack of the one that stayed when it does not.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
