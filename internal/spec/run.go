package spec

import (
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/Andrew-Sem/gc/internal/journal"
	"github.com/Andrew-Sem/gc/internal/web"
)

func (s *Scenario) run(t *testing.T) {
	j := &journal.Journal{}
	j.Append(s.given...)
	before := len(j.Facts())

	response := httptest.NewRecorder()
	web.New(j, &fakeGitHub{sees: s.githubSees}).ServeHTTP(response, s.request.httpRequest())

	if s.status != 0 && response.Code != s.status {
		t.Errorf("ждали статус %d, пришёл %d", s.status, response.Code)
	}

	written := j.Facts()[before:]
	if !slices.Equal(written, s.expected) {
		t.Errorf("ждали факты %#v, записано %#v", s.expected, written)
	}
}
