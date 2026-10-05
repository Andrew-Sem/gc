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

type callCountMismatch struct {
	method string
	got    int
}

func (p callCountMismatch) String() string {
	return fmt.Sprintf("expected one %s call, got %d", p.method, p.got)
}

func (s *Scenario) shapeProblems() []problem {
	calls := []struct {
		method string
		count  int
	}{
		{"When", s.whenCalls},
		{"ExpectStatus", s.statusCalls},
		{"ExpectFacts", s.factsCalls},
	}
	var problems []problem
	for _, c := range calls {
		if c.count != 1 {
			problems = append(problems, callCountMismatch{method: c.method, got: c.count})
		}
	}
	return problems
}

func (s *Scenario) run(newApp func(*journal.Journal, web.GitHub) http.Handler) []problem {
	if problems := s.shapeProblems(); len(problems) > 0 {
		return problems
	}

	var problems []problem
	j := &journal.Journal{}
	j.Append(s.given...)
	before := len(j.Facts())

	response := httptest.NewRecorder()
	newApp(j, &fakeGitHub{sees: s.githubSees}).ServeHTTP(response, s.request.httpRequest())

	if response.Code != s.status {
		problems = append(problems, statusMismatch{expected: s.status, got: response.Code})
	}

	written := j.Facts()[before:]
	if !slices.Equal(written, s.expected) {
		problems = append(problems, factsMismatch{expected: s.expected, got: written})
	}
	return problems
}
