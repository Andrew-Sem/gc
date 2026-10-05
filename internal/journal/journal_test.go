package journal

import (
	"testing"

	"github.com/Andrew-Sem/gc/internal/domain"
)

func TestFactsCannotRewriteHistory(t *testing.T) {
	connected := domain.RepositoryConnected{FullName: "acme/api"}

	j := &Journal{}
	j.Append(connected)
	j.Facts()[0] = domain.RepositoryDisconnected{FullName: "acme/api"}

	if got := j.Facts()[0]; got != connected {
		t.Errorf("first fact is %#v after rewriting the returned slice, want %#v", got, connected)
	}
}
