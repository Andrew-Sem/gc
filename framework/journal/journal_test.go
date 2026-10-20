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
