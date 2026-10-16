package virtualtime_test

import (
	"testing"

	"go.uber.org/goleak"
)

// The clock runs the actors of a test on the test's own thread, and a timer of the
// wall clock gives up its goroutine when it is cancelled: this is what holds the
// tests of the package to both, with the stack of the goroutine that stayed when a
// test leaves one behind.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
