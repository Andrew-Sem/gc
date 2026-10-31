package spec

import (
	"fmt"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/synctest"

	"github.com/Andrew-Sem/gc/framework/journal"
)

type Feature[F comparable] struct {
	mu       sync.Mutex
	t        *testing.T
	contract string
	clauses  clauses
	covered  []string
	failed   []string
}

type group struct {
	id, title string
}

type clauses struct {
	groups []group
	leaves []string
}

var clausePattern = regexp.MustCompile(`^(#+ )?([A-Z]+-[0-9]+(?:\.[0-9]+)*)\. (.*)$`)

func NewFeature[F comparable](t *testing.T, contract string) *Feature[F] {
	text, err := os.ReadFile(contract)
	if err != nil {
		t.Fatalf("cannot read contract: %v", err)
	}
	c, problems := parseClauses(string(text))
	for _, p := range problems {
		t.Errorf("contract %s: %s", contract, p)
	}
	f := &Feature[F]{t: t, contract: contract, clauses: c}
	t.Cleanup(func() {
		f.mu.Lock()
		defer f.mu.Unlock()
		for _, line := range groupReport(f.clauses, f.covered) {
			t.Log(line)
		}
		t.Logf("uncovered: %v", uncovered(f.clauses.leaves, f.covered))
		t.Logf("failing: %v", linked(f.clauses.leaves, f.failed))
	})
	return f
}

func parseClauses(text string) (clauses, []string) {
	var c clauses
	var problems []string
	var area, requirement string
	seen := map[string]bool{}
	for _, line := range strings.Split(text, "\n") {
		m := clausePattern.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		heading, id, rest := m[1] != "", m[2], strings.TrimSpace(m[3])
		if seen[id] {
			problems = append(problems, fmt.Sprintf("%s appears twice", id))
			continue
		}
		seen[id] = true
		switch levels := strings.Count(id, ".") + 1; {
		case heading && levels == 1:
			area, requirement = id, ""
			c.groups = append(c.groups, group{id: id, title: rest})
		case heading && levels == 2:
			if !strings.HasPrefix(id, area+".") {
				problems = append(problems, fmt.Sprintf("%s is not under heading %s", id, parentOf(id)))
			}
			requirement = id
			c.groups = append(c.groups, group{id: id, title: rest})
		case !heading && levels == 3:
			if requirement == "" || !strings.HasPrefix(id, requirement+".") {
				problems = append(problems, fmt.Sprintf("%s is not under heading %s", id, parentOf(id)))
			}
			c.leaves = append(c.leaves, id)
		default:
			problems = append(problems, fmt.Sprintf("%s must be a heading with one or two levels or a clause with three levels", id))
		}
	}
	for _, g := range c.groups {
		if len(leavesUnder(c.leaves, g.id)) == 0 {
			problems = append(problems, fmt.Sprintf("%s has no clauses", g.id))
		}
	}
	return c, problems
}

func parentOf(id string) string {
	return id[:strings.LastIndex(id, ".")]
}

func leavesUnder(leaves []string, groupID string) []string {
	var under []string
	for _, id := range leaves {
		if strings.HasPrefix(id, groupID+".") {
			under = append(under, id)
		}
	}
	return under
}

func groupReport(c clauses, covered []string) []string {
	var lines []string
	for _, g := range c.groups {
		under := leavesUnder(c.leaves, g.id)
		linked := len(under) - len(uncovered(under, covered))
		status := "partial"
		switch linked {
		case 0:
			status = "none"
		case len(under):
			status = "complete"
		}
		lines = append(lines, fmt.Sprintf("%s %s: %s, %d of %d", g.id, g.title, status, linked, len(under)))
	}
	return lines
}

func linked(clauses, ids []string) []string {
	var in []string
	for _, id := range clauses {
		if slices.Contains(ids, id) {
			in = append(in, id)
		}
	}
	return in
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

type Custom interface {
	Covered() []string
	Run() []Problem
}

func (f *Feature[F]) Run(name string, describe func(s *Scenario[F]), newApp func(*journal.Journal[F]) http.Handler) {
	f.t.Helper()
	f.run(name, true, func() ([]string, func() []Problem) {
		s := &Scenario[F]{}
		describe(s)
		return s.covers, func() []Problem { return s.Run(newApp) }
	})
}

func (f *Feature[F]) RunCustom(name string, describe func() Custom) {
	f.t.Helper()
	f.run(name, false, func() ([]string, func() []Problem) {
		c := describe()
		return c.Covered(), c.Run
	})
}

func (f *Feature[F]) run(name string, virtualTime bool, describe func() ([]string, func() []Problem)) {
	f.t.Helper()
	f.t.Run(name, func(t *testing.T) {
		t.Helper()
		if virtualTime {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				t.Helper()
				f.check(t, describe)
			})
			return
		}
		f.check(t, describe)
	})
}

func (f *Feature[F]) check(t *testing.T, describe func() ([]string, func() []Problem)) {
	t.Helper()
	covers, run := describe()
	for _, id := range covers {
		switch {
		case slices.Contains(f.clauses.leaves, id):
		case slices.ContainsFunc(f.clauses.groups, func(g group) bool { return g.id == id }):
			t.Errorf("%s is a heading in contract %s; cover its clauses instead", id, f.contract)
		default:
			t.Errorf("clause %s is not in contract %s", id, f.contract)
		}
	}
	f.mu.Lock()
	f.covered = append(f.covered, covers...)
	f.mu.Unlock()
	problems := run()
	if len(problems) > 0 {
		f.mu.Lock()
		f.failed = append(f.failed, covers...)
		f.mu.Unlock()
	}
	for _, problem := range problems {
		t.Errorf("%s", problem)
	}
}
