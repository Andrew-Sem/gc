package spec

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"

	"github.com/Andrew-Sem/gc/internal/domain"
	"github.com/Andrew-Sem/gc/internal/journal"
	"github.com/Andrew-Sem/gc/internal/web"
)

func factList(facts ...domain.Fact) string {
	var parts []string
	for _, f := range facts {
		parts = append(parts, fmt.Sprintf("%T%+v", f, f))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

type problem interface {
	String() string
}

type statusMismatch struct {
	expected, got int
}

func (p statusMismatch) String() string {
	return fmt.Sprintf("expected status %d, got %d", p.expected, p.got)
}

type factsMismatch struct {
	expected, got []domain.Fact
}

func (p factsMismatch) String() string {
	return fmt.Sprintf("expected facts %s, got %s", factList(p.expected...), factList(p.got...))
}

func (s *Scenario) run(newApp func(*journal.Journal, web.GitHub) http.Handler) []problem {
	var problems []problem
	j := &journal.Journal{}
	j.Append(s.given...)
	before := len(j.Facts())

	response := httptest.NewRecorder()
	newApp(j, &fakeGitHub{sees: s.githubSees}).ServeHTTP(response, s.request.httpRequest())

	if s.status != 0 && response.Code != s.status {
		problems = append(problems, statusMismatch{expected: s.status, got: response.Code})
	}

	written := j.Facts()[before:]
	if !slices.Equal(written, s.expected) {
		problems = append(problems, factsMismatch{expected: s.expected, got: written})
	}
	return problems
}
