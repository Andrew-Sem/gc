package spec

import (
	"fmt"
	"net/http"
	"reflect"
	"testing"

	"github.com/Andrew-Sem/gc/internal/domain"
	"github.com/Andrew-Sem/gc/internal/journal"
	"github.com/Andrew-Sem/gc/internal/web"
)

func TestRunFindsNothingInMatchingScenario(t *testing.T) {
	disconnected := domain.RepositoryDisconnected{FullName: "acme/api"}

	s := &Scenario{}
	s.When(Post("/repositories/acme/api/disconnect")).
		ExpectStatus(http.StatusOK).
		ExpectFacts(disconnected)

	writesExpectedFact := func(j *journal.Journal, _ web.GitHub) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			j.Append(disconnected)
			w.WriteHeader(http.StatusOK)
		})
	}

	if problems := s.run(writesExpectedFact); len(problems) != 0 {
		t.Errorf("run returned %q, want no problems", problems)
	}
}

func TestRunFindsMissingRoute(t *testing.T) {
	s := &Scenario{}
	s.When(Post("/repositories/acme/api/disconnect")).
		ExpectStatus(http.StatusOK).
		ExpectFacts()

	noRoutes := func(*journal.Journal, web.GitHub) http.Handler {
		return http.NewServeMux()
	}

	problems := s.run(noRoutes)
	want := []problem{statusMismatch{expected: http.StatusOK, got: http.StatusNotFound}}
	if !reflect.DeepEqual(problems, want) {
		t.Errorf("run returned %q, want %q", problems, want)
	}
}

func TestRunFindsWrongFacts(t *testing.T) {
	connected := domain.RepositoryConnected{FullName: "acme/api"}
	disconnected := domain.RepositoryDisconnected{FullName: "acme/api"}

	s := &Scenario{}
	s.When(Post("/repositories/acme/api/disconnect")).
		ExpectStatus(http.StatusOK).
		ExpectFacts(disconnected)

	writesWrongFact := func(j *journal.Journal, _ web.GitHub) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			j.Append(connected)
			w.WriteHeader(http.StatusOK)
		})
	}

	problems := s.run(writesWrongFact)
	want := []problem{factsMismatch{expected: []domain.Fact{disconnected}, got: []domain.Fact{connected}}}
	if !reflect.DeepEqual(problems, want) {
		t.Errorf("run returned %q, want %q", problems, want)
	}
}

func TestFactList(t *testing.T) {
	tests := []struct {
		name  string
		facts []domain.Fact
		want  string
	}{
		{"no facts", nil, "[]"},
		{"one fact", []domain.Fact{
			domain.RepositoryConnected{FullName: "acme/api"},
		}, "[domain.RepositoryConnected{FullName:acme/api}]"},
		{"two facts", []domain.Fact{
			domain.RepositoryConnected{FullName: "acme/api"},
			domain.RepositoryDisconnected{FullName: "acme/api"},
		}, "[domain.RepositoryConnected{FullName:acme/api}, domain.RepositoryDisconnected{FullName:acme/api}]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := factList(tt.facts...); got != tt.want {
				t.Errorf("factList() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRunReportsStatusThenFacts(t *testing.T) {
	connected := domain.RepositoryConnected{FullName: "acme/api"}
	disconnected := domain.RepositoryDisconnected{FullName: "acme/api"}

	s := &Scenario{}
	s.When(Post("/repositories/acme/api/disconnect")).
		ExpectStatus(http.StatusOK).
		ExpectFacts(disconnected)

	failsAndWritesWrongFact := func(j *journal.Journal, _ web.GitHub) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			j.Append(connected)
			w.WriteHeader(http.StatusInternalServerError)
		})
	}

	problems := s.run(failsAndWritesWrongFact)
	want := []problem{
		statusMismatch{expected: http.StatusOK, got: http.StatusInternalServerError},
		factsMismatch{expected: []domain.Fact{disconnected}, got: []domain.Fact{connected}},
	}
	if !reflect.DeepEqual(problems, want) {
		t.Errorf("run returned %q, want %q", problems, want)
	}
}

func TestRunAcceptsExplicitlyEmptyFacts(t *testing.T) {
	s := &Scenario{}
	s.When(Get("/repositories")).
		ExpectStatus(http.StatusOK).
		ExpectFacts()

	writesNothing := func(*journal.Journal, web.GitHub) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
	}

	if problems := s.run(writesNothing); len(problems) != 0 {
		t.Errorf("run returned %q, want no problems", problems)
	}
}

func TestRunRejectsMalformedScenario(t *testing.T) {
	tests := []struct {
		name     string
		describe func(s *Scenario)
		want     []problem
	}{
		{"no When", func(s *Scenario) {
			s.ExpectStatus(http.StatusOK).ExpectFacts()
		}, []problem{callCountMismatch{method: "When", got: 0}}},
		{"two Whens", func(s *Scenario) {
			s.When(Get("/repositories")).
				When(Get("/repositories")).
				ExpectStatus(http.StatusOK).
				ExpectFacts()
		}, []problem{callCountMismatch{method: "When", got: 2}}},
		{"no ExpectStatus", func(s *Scenario) {
			s.When(Get("/repositories")).ExpectFacts()
		}, []problem{callCountMismatch{method: "ExpectStatus", got: 0}}},
		{"no ExpectFacts", func(s *Scenario) {
			s.When(Get("/repositories")).ExpectStatus(http.StatusOK)
		}, []problem{callCountMismatch{method: "ExpectFacts", got: 0}}},
		{"nothing at all", func(s *Scenario) {}, []problem{
			callCountMismatch{method: "When", got: 0},
			callCountMismatch{method: "ExpectStatus", got: 0},
			callCountMismatch{method: "ExpectFacts", got: 0},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Scenario{}
			tt.describe(s)

			mustNotStart := func(*journal.Journal, web.GitHub) http.Handler {
				t.Error("run started the app for a malformed scenario")
				return http.NewServeMux()
			}

			if problems := s.run(mustNotStart); !reflect.DeepEqual(problems, tt.want) {
				t.Errorf("run returned %q, want %q", problems, tt.want)
			}
		})
	}
}

func TestProblemString(t *testing.T) {
	tests := []struct {
		name    string
		problem problem
		want    string
	}{
		{"status mismatch", statusMismatch{expected: 200, got: 404}, "expected status 200, got 404"},
		{"facts mismatch", factsMismatch{
			expected: []domain.Fact{domain.RepositoryDisconnected{FullName: "acme/api"}},
			got:      nil,
		}, "expected facts [domain.RepositoryDisconnected{FullName:acme/api}], got []"},
		{"call count mismatch", callCountMismatch{method: "When", got: 2}, "expected one When call, got 2"},
		{"github asked", githubAsked{got: []string{"acme/api"}}, "expected GitHub not to be asked, got [acme/api]"},
		{"element missing", elementMissing{selector: "form"}, "expected element form, found none"},
		{"invalid selector", invalidSelector{selector: "form[", err: "boom"}, "invalid selector form[: boom"},
		{"unreadable page", unreadablePage{err: "boom"}, "cannot read the page: boom"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.problem.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRunFindsGitHubAsked(t *testing.T) {
	s := &Scenario{}
	s.When(Post("/repositories")).
		ExpectStatus(http.StatusOK).
		ExpectFacts().
		ExpectGitHubNotAsked()

	asksGitHub := func(_ *journal.Journal, gh web.GitHub) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gh.CanAccess("acme/api")
			w.WriteHeader(http.StatusOK)
		})
	}

	problems := s.run(asksGitHub)
	want := []problem{githubAsked{got: []string{"acme/api"}}}
	if !reflect.DeepEqual(problems, want) {
		t.Errorf("run returned %q, want %q", problems, want)
	}
}

func TestRunChecksElements(t *testing.T) {
	tests := []struct {
		name     string
		selector string
		want     []problem
	}{
		{"element found", `form input[name="repo"]`, nil},
		{"element missing", `form input[name="owner"]`, []problem{
			elementMissing{selector: `form input[name="owner"]`},
		}},
		{"invalid selector", `form[`, []problem{
			invalidSelector{selector: `form[`, err: "expected identifier, found EOF instead"},
		}},
	}
	servesForm := func(*journal.Journal, web.GitHub) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `<form><input name="repo"></form>`)
		})
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Scenario{}
			s.When(Get("/repositories")).
				ExpectStatus(http.StatusOK).
				ExpectFacts().
				ExpectElement(tt.selector)

			if problems := s.run(servesForm); !reflect.DeepEqual(problems, tt.want) {
				t.Errorf("run returned %q, want %q", problems, tt.want)
			}
		})
	}
}
