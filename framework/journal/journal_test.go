package journal

import (
	"sync"
	"testing"
)

func TestFactsCannotRewriteHistory(t *testing.T) {
	connected := "added"

	j := &Journal[string]{}
	j.Append(connected)
	j.Facts()[0] = "removed"

	if got := j.Facts()[0]; got != connected {
		t.Errorf("first fact is %#v after rewriting the returned slice, want %#v", got, connected)
	}
}

func received(changes <-chan struct{}) bool {
	select {
	case <-changes:
		return true
	default:
		return false
	}
}

func TestSubscriberHearsOfLaterFacts(t *testing.T) {
	j := &Journal[string]{}
	j.Append("earlier")
	changes, cancel := j.Subscribe()
	defer cancel()

	if received(changes) {
		t.Error("subscriber heard of a fact from before it subscribed")
	}
	j.Append("later")
	if !received(changes) {
		t.Error("subscriber did not hear of a new fact")
	}
	if received(changes) {
		t.Error("subscriber heard of one fact twice")
	}
}

func TestBusySubscriberStillHearsOfTheLastFact(t *testing.T) {
	j := &Journal[string]{}
	changes, cancel := j.Subscribe()
	defer cancel()

	j.Append("first")
	j.Append("second")
	if !received(changes) {
		t.Error("subscriber did not hear of the facts")
	}
	if received(changes) {
		t.Error("two facts in a row made two signals, want them merged")
	}
	j.Append("third")
	if !received(changes) {
		t.Error("subscriber did not hear of a fact after reading a signal")
	}
}

func TestEverySubscriberHears(t *testing.T) {
	j := &Journal[string]{}
	first, cancelFirst := j.Subscribe()
	defer cancelFirst()
	second, cancelSecond := j.Subscribe()
	defer cancelSecond()

	j.Append("fact")
	if !received(first) || !received(second) {
		t.Error("a subscriber did not hear of the fact")
	}
}

func TestCancelledSubscriberHearsNothing(t *testing.T) {
	j := &Journal[string]{}
	changes, cancel := j.Subscribe()
	cancel()

	j.Append("fact")
	if received(changes) {
		t.Error("a cancelled subscriber heard of a fact")
	}
}

func TestJournalAcceptsConcurrentUse(t *testing.T) {
	j := &Journal[int]{}
	var wg sync.WaitGroup
	for i := range 50 {
		wg.Go(func() {
			j.Append(i)
			_ = j.Facts()
		})
	}
	wg.Wait()
	if got := len(j.Facts()); got != 50 {
		t.Errorf("journal has %d facts, want 50", got)
	}
}
