package spec

import (
	"net/http"
	"os"
	"regexp"
	"slices"
	"testing"

	"github.com/Andrew-Sem/gc/framework/journal"
)

type Feature[F comparable] struct {
	t        *testing.T
	contract string
	clauses  []string
	covered  []string
}

var clausePattern = regexp.MustCompile(`(?m)^([A-Z]+-[0-9.]*[0-9])\. `)

func NewFeature[F comparable](t *testing.T, contract string) *Feature[F] {
	text, err := os.ReadFile(contract)
	if err != nil {
		t.Fatalf("cannot read contract: %v", err)
	}
	f := &Feature[F]{t: t, contract: contract, clauses: parseClauses(string(text))}
	t.Cleanup(func() {
		t.Logf("uncovered: %v", uncovered(f.clauses, f.covered))
		t.Logf("covered: %v", f.covered)
	})
	return f
}

func parseClauses(text string) []string {
	var clauses []string
	for _, m := range clausePattern.FindAllStringSubmatch(text, -1) {
		clauses = append(clauses, m[1])
	}
	return clauses
}

func uncovered(clauses, covered []string) []string {
	var rest []string
	for _, id := range clauses {
		if !slices.Contains(covered, id) {
			rest = append(rest, id)
		}
	}
	return rest
}

func (f *Feature[F]) Run(name string, describe func(s *Scenario[F]), newApp func(*journal.Journal[F]) http.Handler) {
	f.t.Run(name, func(t *testing.T) {
		s := &Scenario[F]{}
		describe(s)
		for _, id := range s.covers {
			if !slices.Contains(f.clauses, id) {
				t.Errorf("clause %s is not in contract %s", id, f.contract)
			}
		}
		f.covered = append(f.covered, s.covers...)
		for _, problem := range s.Run(newApp) {
			t.Errorf("%s", problem)
		}
	})
}
