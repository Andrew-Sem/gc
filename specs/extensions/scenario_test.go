package extensions

import (
	"net/http"
	"reflect"
	"testing"

	"github.com/Andrew-Sem/gc/framework/spec"
	"github.com/Andrew-Sem/gc/specs/contract"
)

func TestRunFindsGitHubAsked(t *testing.T) {
	s := newScenario(&spec.Scenario[contract.Fact]{})
	s.When(spec.Post("/repositories")).
		ExpectStatus(http.StatusOK).
		ExpectFacts()
	s.ExpectGitHubNotAsked()

	asksGitHub := func(deps contract.Dependencies) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			deps.GitHub.CanAccess("acme/api")
			w.WriteHeader(http.StatusOK)
		})
	}

	problems := s.Run(s.app(asksGitHub))
	want := []spec.Problem{githubAsked{got: []string{"acme/api"}}}
	if !reflect.DeepEqual(problems, want) {
		t.Errorf("run returned %q, want %q", problems, want)
	}
}

func TestGitHubAskedString(t *testing.T) {
	p := githubAsked{got: []string{"acme/api"}}
	want := "expected GitHub not to be asked, got [acme/api]"
	if got := p.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}
