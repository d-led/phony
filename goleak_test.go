package phony

import (
	"testing"

	"go.uber.org/goleak"
)

// The worker of an Inbox goes away when its queue is drained, and the timers of an
// actor that is cancelled go with it: this is what holds the tests of the package
// to that, with the stack of the goroutine that stayed when one does.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
