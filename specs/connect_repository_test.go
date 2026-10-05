package specs

import (
	"net/http"
	"testing"

	"github.com/Andrew-Sem/gc/internal/domain"
	"github.com/Andrew-Sem/gc/internal/spec"
)

func TestConnectRepository(t *testing.T) {
	f := spec.NewFeature(t, "connect-repository.md")
	f.Run("page opens", func(s *spec.Scenario) {
		s.When(spec.Get("/repositories")).
			Then("the operator opens the repositories page").
			ExpectStatus(http.StatusOK)
	})

	f.Run("connect a repository the token can see", func(s *spec.Scenario) {
		s.Covers("REPO-1.1").
			GivenGitHubSees("acme/api").
			When(spec.Post("/repositories").WithForm("repo", "acme/api")).
			Then("the operator connects a repository").
			ExpectFacts(domain.RepositoryConnected{FullName: "acme/api"})
	})

	f.Run("connect a repository the token cannot see", func(s *spec.Scenario) {
		s.Covers("REPO-1.2").
			When(spec.Post("/repositories").WithForm("repo", "acme/api")).
			Then("the operator tries to connect a repository the token cannot see").
			ExpectStatus(http.StatusUnprocessableEntity).
			ExpectFacts()
	})

	f.Run("disconnect a connected repository", func(s *spec.Scenario) {
		s.Covers("REPO-3.1").
			Given(domain.RepositoryConnected{FullName: "acme/api"}).
			When(spec.Post("/repositories/acme/api/disconnect")).
			Then("the operator disconnects a connected repository").
			ExpectFacts(domain.RepositoryDisconnected{FullName: "acme/api"})
	})

	f.Run("disconnect an already disconnected repository", func(s *spec.Scenario) {
		s.Covers("REPO-3.2").
			Given(
				domain.RepositoryConnected{FullName: "acme/api"},
				domain.RepositoryDisconnected{FullName: "acme/api"},
			).
			When(spec.Post("/repositories/acme/api/disconnect")).
			Then("the operator presses disconnect again on a stale page").
			ExpectFacts()
	})
}
