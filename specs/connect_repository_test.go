package specs

import (
	"net/http"
	"testing"

	"github.com/Andrew-Sem/gc/internal/domain"
	"github.com/Andrew-Sem/gc/internal/spec"
)

func TestConnectRepository(t *testing.T) {
	f := spec.NewFeature(t, "connect-repository.md")
	f.Run("repositories page shows the connect form", func(s *spec.Scenario) {
		s.Covers("REPO-1").
			When(spec.Get("/repositories")).
			Then("the connect form is shown").
			ExpectStatus(http.StatusOK).
			ExpectFacts().
			ExpectElement(`form[hx-post="/repositories"] input[name="repo"]`)
	})

	f.Run("connect a repository the token can see", func(s *spec.Scenario) {
		s.Covers("REPO-1.1").
			GivenGitHubSees("acme/api").
			When(spec.Post("/repositories").WithForm("repo", "acme/api")).
			Then("the repository is connected").
			ExpectStatus(http.StatusOK).
			ExpectFacts(domain.RepositoryConnected{FullName: "acme/api"})
	})

	f.Run("connect a repository the token cannot see", func(s *spec.Scenario) {
		s.Covers("REPO-1.2").
			When(spec.Post("/repositories").WithForm("repo", "acme/api")).
			Then("the request is rejected and nothing is connected").
			ExpectStatus(http.StatusUnprocessableEntity).
			ExpectFacts()
	})

	f.Run("connect an already connected repository", func(s *spec.Scenario) {
		s.Covers("REPO-1.3").
			GivenGitHubSees("acme/api").
			Given(domain.RepositoryConnected{FullName: "acme/api"}).
			When(spec.Post("/repositories").WithForm("repo", "acme/api")).
			Then("nothing more is recorded and GitHub is not asked").
			ExpectStatus(http.StatusOK).
			ExpectFacts().
			ExpectGitHubNotAsked()
	})

	f.Run("connect a malformed repository name", func(s *spec.Scenario) {
		s.Covers("REPO-1.4").
			GivenGitHubSees("acmeapi").
			When(spec.Post("/repositories").WithForm("repo", "acmeapi")).
			Then("the request is rejected and GitHub is not asked").
			ExpectStatus(http.StatusUnprocessableEntity).
			ExpectFacts().
			ExpectGitHubNotAsked()
	})

	f.Run("connect while GitHub does not answer", func(s *spec.Scenario) {
		s.Covers("REPO-1.5").
			GivenGitHubDoesNotAnswer().
			When(spec.Post("/repositories").WithForm("repo", "acme/api")).
			Then("the request fails and nothing is connected").
			ExpectStatus(http.StatusBadGateway).
			ExpectFacts()
	})

	f.Run("disconnect a connected repository", func(s *spec.Scenario) {
		s.Covers("REPO-3.1").
			Given(domain.RepositoryConnected{FullName: "acme/api"}).
			When(spec.Post("/repositories/acme/api/disconnect")).
			Then("the repository is disconnected").
			ExpectStatus(http.StatusOK).
			ExpectFacts(domain.RepositoryDisconnected{FullName: "acme/api"})
	})

	f.Run("disconnect an already disconnected repository", func(s *spec.Scenario) {
		s.Covers("REPO-3.2").
			Given(
				domain.RepositoryConnected{FullName: "acme/api"},
				domain.RepositoryDisconnected{FullName: "acme/api"},
			).
			When(spec.Post("/repositories/acme/api/disconnect")).
			Then("nothing more is recorded").
			ExpectStatus(http.StatusOK).
			ExpectFacts()
	})

	f.Run("reconnect a disconnected repository", func(s *spec.Scenario) {
		s.Covers("REPO-3.3").
			Given(
				domain.RepositoryConnected{FullName: "acme/api"},
				domain.RepositoryDisconnected{FullName: "acme/api"},
			).
			GivenGitHubSees("acme/api").
			When(spec.Post("/repositories").WithForm("repo", "acme/api")).
			Then("the repository is connected again").
			ExpectStatus(http.StatusOK).
			ExpectFacts(domain.RepositoryConnected{FullName: "acme/api"})
	})
}
