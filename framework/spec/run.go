package spec

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"

	"github.com/Andrew-Sem/gc/framework/journal"
)

func factList[F any](facts ...F) string {
	var parts []string
	for _, f := range facts {
		parts = append(parts, fmt.Sprintf("%T%+v", f, f))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

type Problem interface {
	String() string
}

type statusMismatch struct {
	expected, got int
}

func (p statusMismatch) String() string {
	return fmt.Sprintf("expected status %d, got %d", p.expected, p.got)
}

type factsMismatch[F any] struct {
	expected, got []F
}

func (p factsMismatch[F]) String() string {
	return fmt.Sprintf("expected facts %s, got %s", factList(p.expected...), factList(p.got...))
}

type bodyMissing struct {
	text string
	got  string
}

func (p bodyMissing) String() string {
	return fmt.Sprintf("expected the response to contain %q, got %q", p.text, p.got)
}

type bodyContains struct {
	text string
	got  string
}

func (p bodyContains) String() string {
	return fmt.Sprintf("expected the response not to contain %q, got %q", p.text, p.got)
}

type callCountMismatch struct {
	method string
	got    int
}

func (p callCountMismatch) String() string {
	return fmt.Sprintf("expected one %s call, got %d", p.method, p.got)
}

func (s *Scenario[F]) shapeProblems() []Problem {
	calls := []struct {
		method string
		count  int
	}{
		{"When", s.whenCalls},
		{"ExpectStatus", s.statusCalls},
		{"ExpectFacts", s.factsCalls},
	}
	var problems []Problem
	for _, c := range calls {
		if c.count != 1 {
			problems = append(problems, callCountMismatch{method: c.method, got: c.count})
		}
	}
	return problems
}

func (s *Scenario[F]) Run(newApp func(*journal.Journal[F]) http.Handler) []Problem {
	if problems := s.shapeProblems(); len(problems) > 0 {
		return problems
	}

	var problems []Problem
	j := &journal.Journal[F]{}
	j.Append(s.given...)
	before := len(j.Facts())

	response := httptest.NewRecorder()
	newApp(j).ServeHTTP(response, s.request.httpRequest())

	if response.Code != s.status {
		problems = append(problems, statusMismatch{expected: s.status, got: response.Code})
	}

	written := j.Facts()[before:]
	if !slices.Equal(written, s.expected) {
		problems = append(problems, factsMismatch[F]{expected: s.expected, got: written})
	}

	for _, text := range s.bodyTexts {
		if !strings.Contains(response.Body.String(), text) {
			problems = append(problems, bodyMissing{text: text, got: response.Body.String()})
		}
	}

	for _, text := range s.absentText {
		if strings.Contains(response.Body.String(), text) {
			problems = append(problems, bodyContains{text: text, got: response.Body.String()})
		}
	}

	for _, check := range s.checks {
		if p := check(); p != nil {
			problems = append(problems, p)
		}
	}

	if len(s.elements) > 0 || len(s.noElements) > 0 || len(s.texts) > 0 || len(s.counts) > 0 {
		problems = append(problems, elementProblems(response.Body.String(), s.elements, s.noElements, s.texts, s.counts)...)
	}
	return problems
}
