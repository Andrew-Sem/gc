package extensions

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Andrew-Sem/gc/framework/journal"
	"github.com/Andrew-Sem/gc/framework/spec"
	"github.com/Andrew-Sem/gc/internal/web"
	"github.com/Andrew-Sem/gc/specs/contract"
)

var (
	PullCreatedAt = At("1999-12-31 10:00")
	PullUpdatedAt = At("1999-12-31 11:00")
	PullMergedAt  = At("1999-12-31 12:00")
)

// PullOnGitHub is a PR as GitHub gives it. Mergeable is "true", "false", or "null".
type PullOnGitHub struct {
	Closed         bool
	Draft          bool
	Head           string
	Base           string
	Mergeable      string
	MergeableState string
	UpdatedAt      time.Time
	Unreadable     bool
	Incomplete     bool
}

func OpenPullOnGitHub() PullOnGitHub {
	return PullOnGitHub{Head: "abc123", Base: "main", Mergeable: "true", MergeableState: "clean", UpdatedAt: PullUpdatedAt}
}

// Snapshot is the PrStateChanged that the PR read gives.
func (p PullOnGitHub) Snapshot(repository string, number int) contract.PrStateChanged {
	mergeable := contract.MergeableUnknown
	switch p.Mergeable {
	case "true":
		mergeable = contract.MergeableYes
	case "false":
		mergeable = contract.MergeableNo
	}
	return contract.PrStateChanged{
		Number: number, Repository: repository, HeadSHA: p.Head, Base: p.Base, Draft: p.Draft,
		Open: !p.Closed, Mergeable: mergeable, CreatedAt: PullCreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

type mergeCall struct {
	Repository string
	Number     int
	SHA        string `json:"sha"`
	Method     string `json:"merge_method"`
}

func (s *Scenario) GivenPullOnGitHub(repository string, number int, pull PullOnGitHub) *Scenario {
	s.github.given(func(g *fakeGitHub) {
		if g.pulls == nil {
			g.pulls = map[string]PullOnGitHub{}
		}
		g.pulls[repository+"#"+strconv.Itoa(number)] = pull
	})
	return s
}

func (s *Scenario) GivenRequiredChecks(contexts ...string) *Scenario {
	s.github.given(func(g *fakeGitHub) {
		g.required = contexts
		g.requiredSet = true
	})
	return s
}

func (s *Scenario) GivenRequiredCheckApps(apps ...string) *Scenario {
	s.github.given(func(g *fakeGitHub) {
		g.requiredApps = apps
		g.required = nil
		g.requiredSet = true
	})
	return s
}

func (s *Scenario) GivenBaseBranchUnreadable() *Scenario {
	s.github.given(func(g *fakeGitHub) { g.branchUnreadable = true })
	return s
}

func (s *Scenario) GivenGitHubRefusesMerge(status int) *Scenario {
	s.github.given(func(g *fakeGitHub) { g.mergeStatus = status })
	return s
}

func (s *Scenario) GivenMergeHangs() *Scenario {
	s.github.given(func(g *fakeGitHub) { g.mergeHangs = true })
	return s
}

func (s *Scenario) WhenSwept() *Scenario {
	s.WhenAction(func(j *journal.Journal[contract.Fact]) {
		web.Sweep(context.Background(), s.dependencies(j))
	})
	return s
}

func (s *Scenario) GitHubMerged(repository string, number int, sha string) func() spec.Problem {
	return func() spec.Problem {
		want := []mergeCall{{Repository: repository, Number: number, SHA: sha, Method: "squash"}}
		if got := s.github.mergesSoFar(); !slices.Equal(got, want) {
			return githubMerged{want: want, got: got}
		}
		return nil
	}
}

func (s *Scenario) GitHubMergedNothing() func() spec.Problem {
	return func() spec.Problem {
		if got := s.github.mergesSoFar(); len(got) != 0 {
			return githubMerged{got: got}
		}
		return nil
	}
}

type githubMerged struct {
	want, got []mergeCall
}

func (p githubMerged) String() string {
	return fmt.Sprintf("expected GitHub to be asked to merge %+v, got %+v", p.want, p.got)
}

func (g *fakeGitHub) mergesSoFar() []mergeCall {
	g.mu.Lock()
	defer g.mu.Unlock()
	return slices.Clone(g.merges)
}

func (g *fakeGitHub) servePullsAPI(w http.ResponseWriter, r *http.Request, fail func(int, string)) bool {
	rest, _ := strings.CutPrefix(r.URL.Path, "/repos/")
	parts := strings.Split(rest, "/")
	if len(parts) < 4 {
		return false
	}
	repository := parts[0] + "/" + parts[1]
	switch {
	case r.Method == http.MethodGet && parts[2] == "branches" && len(parts) == 4:
		g.serveBranch(w, fail)
		return true
	case parts[2] != "pulls":
		return false
	}
	number, err := strconv.Atoi(parts[3])
	if err != nil {
		return false
	}
	key := repository + "#" + parts[3]
	g.mu.Lock()
	pull, known := g.pulls[key]
	g.mu.Unlock()
	switch {
	case !known:
		return false
	case r.Method == http.MethodGet && len(parts) == 4:
		servePull(w, pull, number, fail)
	case r.Method == http.MethodPut && len(parts) == 5 && parts[4] == "merge":
		g.merge(w, r, repository, number, key, fail)
	default:
		return false
	}
	return true
}

func servePull(w http.ResponseWriter, pull PullOnGitHub, number int, fail func(int, string)) {
	if pull.Unreadable {
		fail(http.StatusInternalServerError, "Server Error")
		return
	}
	state := "open"
	if pull.Closed {
		state = "closed"
	}
	fields := map[string]any{
		"number":          number,
		"state":           state,
		"draft":           pull.Draft,
		"head":            map[string]string{"sha": pull.Head},
		"base":            map[string]string{"ref": pull.Base},
		"mergeable":       json.RawMessage(pull.Mergeable),
		"mergeable_state": pull.MergeableState,
		"created_at":      PullCreatedAt.Format(time.RFC3339),
		"updated_at":      pull.UpdatedAt.Format(time.RFC3339),
	}
	if pull.Incomplete {
		delete(fields, "head")
	}
	json.NewEncoder(w).Encode(fields)
}

func (g *fakeGitHub) serveBranch(w http.ResponseWriter, fail func(int, string)) {
	g.mu.Lock()
	unreadable, required, apps, set := g.branchUnreadable, g.required, g.requiredApps, g.requiredSet
	g.mu.Unlock()
	if unreadable {
		fail(http.StatusInternalServerError, "Server Error")
		return
	}
	if !set {
		required = []string{"test"}
	}
	checks := []map[string]any{}
	for _, app := range apps {
		checks = append(checks, map[string]any{"context": app, "app_id": 15368})
	}
	if required == nil {
		required = []string{}
	}
	json.NewEncoder(w).Encode(map[string]any{
		"name":      "main",
		"protected": len(required)+len(checks) > 0,
		"protection": map[string]any{
			"enabled": len(required)+len(checks) > 0,
			"required_status_checks": map[string]any{
				"enforcement_level": "non_admins",
				"contexts":          required,
				"checks":            checks,
			},
		},
	})
}

func (g *fakeGitHub) merge(w http.ResponseWriter, r *http.Request, repository string, number int, key string, fail func(int, string)) {
	call := mergeCall{Repository: repository, Number: number}
	json.NewDecoder(r.Body).Decode(&call)
	g.mu.Lock()
	g.merges = append(g.merges, call)
	status, hangs := g.mergeStatus, g.mergeHangs
	g.mu.Unlock()
	if hangs {
		g.hang(r)
		return
	}
	if status != 0 {
		fail(status, http.StatusText(status))
		return
	}
	g.mu.Lock()
	pull := g.pulls[key]
	pull.Closed, pull.UpdatedAt = true, PullMergedAt
	g.pulls[key] = pull
	g.mu.Unlock()
	fmt.Fprint(w, `{"merged":true}`)
}
