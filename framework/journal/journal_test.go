package journal

import (
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
