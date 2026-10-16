package virtualtime_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/d-led/phony"
	"github.com/d-led/phony/virtualtime"
)

// The clock runs the work of an actor itself, on the thread that advances it, so
// the actors of one test are not threads of their own: several of them take their
// turns while the test advances the clock, and the test knows what they did as soon
// as the call returns. The package's TestMain holds the tests to leaving no
// goroutine behind.

// lastTurn is how many turns the pair below takes.
const lastTurn = 20

// Two actors pass one turn back and forth, and the journal has the turns in the
// order they were taken.
func TestTwoActorsPassingATurnTakeItInOrder(t *testing.T) {
	clock := virtualtime.New()
	defer clock.Reset()

	journal := newJournal()
	first := &turnTaker{number: 1, journal: journal}
	second := &turnTaker{number: 2, journal: journal}
	first.other, second.other = second, first

	first.take(1)
	clock.Run() // the actors take their turns; no time passes on the way

	turns := journal.taken()
	if len(turns) != lastTurn {
		t.Fatalf("the pair took %d turns, want %d: %v", len(turns), lastTurn, turns)
	}

	for i, taken := range turns {
		if taken.seq != i+1 {
			t.Fatalf("turn %d came as %v: %v", i+1, taken, turns)
		}
		if who := 1 + i%2; taken.actor != who {
			t.Fatalf("turn %d came from actor %d, want %d: %v", i+1, taken.actor, who, turns)
		}
	}

	if timers, ready := clock.Pending(); timers != 0 || ready != 0 {
		t.Fatalf("the clock still holds %d timers and %d work items, want none", timers, ready)
	}
	if !clock.Now().Equal(time.Unix(0, 0)) {
		t.Fatalf("the turns moved the clock to %v, want it where it was", clock.Now())
	}
}

// Several actors hold their messages for a delay, and the journal has all of them
// once the delay is up: the test decides when that is, and waits for nothing.
func TestSeveralActorsHoldTheirMessagesForTheirOwnDelay(t *testing.T) {
	const (
		actors = 4
		each   = 5
		delay  = time.Second
	)

	clock := virtualtime.New()
	defer clock.Reset()

	journal := newJournal()

	for number := 0; number < actors; number++ {
		holder := &holder{number: number, journal: journal, timer: clock, delay: delay}

		for seq := 1; seq <= each; seq++ {
			holder.hold(seq)
		}
	}

	clock.Advance(delay / 2) // half of the delay: nothing is due yet
	if taken := journal.taken(); len(taken) != 0 {
		t.Fatalf("the actors delivered %v before their delay was up, want nothing", taken)
	}

	clock.Advance(delay / 2) // the other half: the delays are up
	if got, want := theSequencesOf(journal.taken(), actors), countingTo(actors, each); !reflect.DeepEqual(got, want) {
		t.Fatalf("the journal reads %v, want %v", got, want)
	}

	if timers, ready := clock.Pending(); timers != 0 || ready != 0 {
		t.Fatalf("the clock still holds %d timers and %d work items, want none", timers, ready)
	}
}

// turnTaker is one of a pair that passes a turn to the other: it records its own
// turn, then hands the next one over.
type turnTaker struct {
	phony.Inbox
	number  int // which of the pair this is: 1 or 2
	journal *journal
	other   *turnTaker
}

// take takes turn n, and passes n+1 on until the last turn.
func (t *turnTaker) take(n int) {
	t.Act(nil, func() {
		t.journal.note(t.number, n)

		if n < lastTurn {
			t.other.take(n + 1)
		}
	})
}

// holder is an actor that holds every message it is given for a delay, and then
// records it in the journal.
type holder struct {
	phony.Inbox
	number  int
	journal *journal
	timer   virtualtime.Timer
	delay   time.Duration
}

// hold is one message: it reaches the journal once the delay has passed.
func (h *holder) hold(seq int) {
	h.Act(nil, func() { h.timer.After(h.delay, func() { h.journal.note(h.number, seq) }) })
}

// entry is one message a journal took: the actor it came from, and where in that
// actor's own sequence it was.
type entry struct {
	actor int
	seq   int
}

// journal is an actor that records what the other actors of a test did, in the order
// it took the messages.
type journal struct {
	phony.Inbox
	entries []entry
}

func newJournal() *journal { return &journal{} }

// note records one message; the actors of a test send it to the journal.
func (j *journal) note(actor, seq int) {
	j.Act(nil, func() { j.entries = append(j.entries, entry{actor: actor, seq: seq}) })
}

// taken answers what the journal took. The clock has run the actors on the thread
// of the test by the time it returns, which is why this reads the field and does
// not Block for it: Block waits for work that only the test itself can run.
func (j *journal) taken() []entry { return j.entries }

// theSequencesOf answers, per actor, that actor's own messages, in the order the
// journal took them.
func theSequencesOf(taken []entry, actors int) [][]int {
	sequences := make([][]int, actors)

	for _, taken := range taken {
		sequences[taken.actor] = append(sequences[taken.actor], taken.seq)
	}

	return sequences
}

// countingTo answers what each of actors actors sent, in the order it sent them:
// 1, 2, ..., each.
func countingTo(actors, each int) [][]int {
	sequences := make([][]int, actors)

	for actor := range sequences {
		sequences[actor] = make([]int, each)

		for i := range sequences[actor] {
			sequences[actor][i] = i + 1
		}
	}

	return sequences
}
