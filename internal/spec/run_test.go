package spec

import (
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
