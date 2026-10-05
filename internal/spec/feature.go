package spec

import (
	"os"
	"regexp"
	"slices"
	"testing"

	"github.com/Andrew-Sem/gc/internal/web"
)

type Feature struct {
	t        *testing.T
	contract string
	clauses  []string
	covered  []string
}

var clausePattern = regexp.MustCompile(`(?m)^([A-Z]+-[0-9.]*[0-9])\. `)

func NewFeature(t *testing.T, contract string) *Feature {
	text, err := os.ReadFile(contract)
	if err != nil {
		t.Fatalf("cannot read contract: %v", err)
	}
	f := &Feature{t: t, contract: contract, clauses: parseClauses(string(text))}
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

func (f *Feature) Run(name string, describe func(s *Scenario)) {
	f.t.Run(name, func(t *testing.T) {
		s := &Scenario{}
		describe(s)
		for _, id := range s.covers {
			if !slices.Contains(f.clauses, id) {
				t.Errorf("clause %s is not in contract %s", id, f.contract)
			}
		}
		f.covered = append(f.covered, s.covers...)
		for _, problem := range s.run(web.New) {
			t.Errorf("%s", problem)
		}
	})
}
