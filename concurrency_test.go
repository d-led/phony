package phony

import (
	"reflect"
	"sync"
	"testing"
	"time"
)

// This file is the library used the way a concurrent program uses it: several
// senders at once, and an actor that is flooded. What the tests here do not check
// for themselves, the race detector does, and the package's TestMain checks that
// none of it leaves a goroutine behind.

// The messages of one sender arrive in the order that sender sent them, whatever
// the other senders are doing at the same time.
func TestTheMessagesOfOneSenderArriveInOrder(t *testing.T) {
	const (
		senders = 8
		each    = 200
	)

	actor := newRecorder(senders)

	var sent sync.WaitGroup

	for sender := 0; sender < senders; sender++ {
		sent.Add(1)

		go func(sender int) {
			defer sent.Done()

			for seq := 1; seq <= each; seq++ {
				actor.note(sender, seq)
			}
		}(sender)
	}

	sent.Wait()

	for sender, taken := range actor.taken() {
		if !reflect.DeepEqual(taken, countingTo(each)) {
			t.Fatalf("sender %d sent %v and the actor took %v", sender, countingTo(each), taken)
		}
	}
}

// An actor that is flooded takes every message of the flood exactly once, in the
// order it was sent, and the flood finishes: the backpressure of a busy Inbox holds
// the sender back at some point rather than letting it run away with the queue.
func TestAFloodedActorTakesEveryMessageExactlyOnceAndInOrder(t *testing.T) {
	const flood = 1000

	var flooded Inbox
	taken := 0 // written by the flooded actor

	// The flood goes to an actor that is working already: a message that arrives at
	// an idle actor starts it and is taken without backpressure, and it is the busy
	// actor that makes the flooder wait.
	running := make(chan struct{})
	flooded.Act(nil, func() {
		close(running)

		time.Sleep(10 * time.Millisecond)
	})
	<-running

	var flooder Inbox
	pastTheFlood := make(chan struct{})

	flooder.Act(nil, func() {
		for seq := 1; seq <= flood; seq++ {
			seq := seq // Because the flooder's loop moves on without waiting

			flooded.Act(&flooder, func() {
				taken++

				if seq != taken {
					t.Errorf("the flooded actor took message %d as number %d", seq, taken)
				}

				// Slow enough that the actor stays behind the flooder: an actor that
				// catches up between two messages stops being busy, and a message that
				// arrives at it is taken without a wait.
				time.Sleep(50 * time.Microsecond)
			})
		}

		close(pastTheFlood)
	})

	select {
	case <-pastTheFlood:
	case <-time.After(30 * time.Second):
		t.Fatal("the flooder was still sending to the flooded actor after thirty seconds")
	}

	Block(&flooded, func() {}) // the flooded actor has taken everything it was sent

	if taken != flood {
		t.Fatalf("the flooded actor took %d messages, want %d", taken, flood)
	}
}

// recorder is an actor that remembers what each sender sent it, in the order it
// took the messages.
type recorder struct {
	Inbox
	perSender [][]int
}

func newRecorder(senders int) *recorder {
	return &recorder{perSender: make([][]int, senders)}
}

// note is one message: the number of the sender, and where in that sender's own
// sequence the message was.
func (r *recorder) note(sender, seq int) {
	r.Act(nil, func() { r.perSender[sender] = append(r.perSender[sender], seq) })
}

// taken answers what the actor took; a test reads it once the actor has run
// everything it was sent.
func (r *recorder) taken() [][]int {
	Block(r, func() {})

	return r.perSender
}

// countingTo answers what a sender of n messages sends: 1, 2, ..., n.
func countingTo(n int) []int {
	counted := make([]int, n)
	for i := range counted {
		counted[i] = i + 1
	}

	return counted
}
