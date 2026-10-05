package spec

import (
	"os"
	"regexp"
	"slices"
	"testing"
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
		t.Fatalf("не прочитать договор: %v", err)
	}
	var clauses []string
	for _, m := range clausePattern.FindAllStringSubmatch(string(text), -1) {
		clauses = append(clauses, m[1])
	}
	f := &Feature{t: t, contract: contract, clauses: clauses}
	t.Cleanup(func() {
		var uncovered []string
		for _, id := range f.clauses {
			if !slices.Contains(f.covered, id) {
				uncovered = append(uncovered, id)
			}
		}
		t.Logf("без спек: %v", uncovered)
		t.Logf("со спеками: %v", f.covered)
	})
	return f
}

func (f *Feature) Run(name string, describe func(s *Scenario)) {
	f.t.Run(name, func(t *testing.T) {
		s := &Scenario{}
		describe(s)
		for _, id := range s.covers {
			if !slices.Contains(f.clauses, id) {
				t.Errorf("пункта %s нет в договоре %s", id, f.contract)
			}
		}
		f.covered = append(f.covered, s.covers...)
		s.run(t)
	})
}
