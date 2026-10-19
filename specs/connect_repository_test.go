package specs

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/Andrew-Sem/gc/framework/spec"
	"github.com/Andrew-Sem/gc/specs/contract"
	"github.com/Andrew-Sem/gc/specs/extensions"
)

func TestConnectRepository(t *testing.T) {
	f := extensions.NewFeature(t, "connect-repository.md")
	f.Run("repositories page shows the connect form", func(s *extensions.Scenario) {
		s.Covers("REPO-1.1.1").
			When(spec.Get("/repositories")).
			Then("the connect form is shown and wired to htmx").
			ExpectStatus(http.StatusOK).
			ExpectFacts().
			ExpectElement(`form[hx-post="/repositories"] input[name="repo"]`).
			ExpectElement(`script[src="/static/htmx.min.js"]`).
			ExpectElement(`meta[name="htmx-config"]`).
			ExpectElement(`#repositories-panel[hx-target="#repositories-panel"][hx-swap="outerHTML"] form`)
	})

	f.Run("connect a repository the token can see", func(s *extensions.Scenario) {
		s.GivenGitHubSees("acme/api").
			Covers("REPO-1.2.1", "REPO-1.2.2").
			When(spec.Post("/repositories").WithForm("repo", "acme/api")).
			Then("the repository is connected and listed").
			ExpectStatus(http.StatusOK).
			ExpectFacts(contract.RepositoryConnected{FullName: "acme/api"}).
			ExpectElement(`#repositories-panel li[data-repository="acme/api"]`).
			ExpectNoElement(`#repositories-panel [role="alert"]`)
	})

	f.Run("connect a repository the token cannot see", func(s *extensions.Scenario) {
		s.Covers("REPO-1.3.1").
			When(spec.Post("/repositories").WithForm("repo", "acme/api")).
			Then("the request is rejected and nothing is connected").
			ExpectStatus(http.StatusUnprocessableEntity).
			ExpectFacts().
			ExpectText(`#repositories-panel [role="alert"]`, "No access to this repository")
	})

	f.Run("connect an already connected repository", func(s *extensions.Scenario) {
		s.GivenGitHubSees("acme/api").
			Covers("REPO-1.4.1", "REPO-1.4.2").
			Given(contract.RepositoryConnected{FullName: "acme/api"}).
			When(spec.Post("/repositories").WithForm("repo", "acme/api")).
			Then("nothing more is recorded and GitHub is not asked").
			ExpectStatus(http.StatusOK).
			ExpectFacts().
			ExpectElement(`#repositories-panel li[data-repository="acme/api"]`).
			Expect(s.GitHubNotAsked())
	})

	for _, name := range []string{"Acme/API", " acme/api ", " Acme/API "} {
		f.Run("connect a normalized repository name "+strconv.Quote(name), func(s *extensions.Scenario) {
			s.GivenGitHubSees("acme/api").
				Covers("REPO-1.2.1", "REPO-1.2.2", "REPO-1.7.1", "REPO-1.7.2").
				When(spec.Post("/repositories").WithForm("repo", name)).
				Then("the normalized repository name is recorded and displayed").
				ExpectStatus(http.StatusOK).
				ExpectFacts(contract.RepositoryConnected{FullName: "acme/api"}).
				ExpectElement(`#repositories-panel li[data-repository="acme/api"]`).
				ExpectNoElement(`#repositories-panel [role="alert"]`)
		})
	}

	for _, name := range []string{"Acme/API", " acme/api ", " Acme/API "} {
		f.Run("connect an already connected normalized repository name "+strconv.Quote(name), func(s *extensions.Scenario) {
			s.Covers("REPO-1.4.1", "REPO-1.4.2", "REPO-1.7.3").
				Given(contract.RepositoryConnected{FullName: "acme/api"}).
				When(spec.Post("/repositories").WithForm("repo", name)).
				Then("nothing more is recorded and GitHub is not asked").
				ExpectStatus(http.StatusOK).
				ExpectFacts().
				ExpectElement(`#repositories-panel li[data-repository="acme/api"]`).
				ExpectNoElement(`#repositories-panel [role="alert"]`).
				Expect(s.GitHubNotAsked())
		})
	}

	for _, name := range []string{"acmeapi", "/api", "acme/", "acme/api/extra", "", "ac me/api", "acme/ap i", "acme /api", "acme/ api", "   "} {
		f.Run("reject malformed repository name "+strconv.Quote(name), func(s *extensions.Scenario) {
			s.GivenGitHubSees(name).
				Covers("REPO-1.5.1", "REPO-1.5.2").
				When(spec.Post("/repositories").WithForm("repo", name)).
				Then("the request is rejected and GitHub is not asked").
				ExpectStatus(http.StatusUnprocessableEntity).
				ExpectFacts().
				ExpectText(`#repositories-panel [role="alert"]`, "Enter the repository as owner/name").
				Expect(s.GitHubNotAsked())
		})
	}

	f.Run("connect while GitHub does not answer", func(s *extensions.Scenario) {
		s.GivenGitHubDoesNotAnswer().
			Covers("REPO-1.6.1").
			When(spec.Post("/repositories").WithForm("repo", "acme/api")).
			Then("the request fails and nothing is connected").
			ExpectStatus(http.StatusBadGateway).
			ExpectFacts().
			ExpectText(`#repositories-panel [role="alert"]`, "Could not check access: GitHub did not respond")
	})

	f.Run("connect while GitHub rejects the token", func(s *extensions.Scenario) {
		s.GivenGitHubRejectsToken().
			Covers("REPO-1.8.1").
			When(spec.Post("/repositories").WithForm("repo", "acme/api")).
			Then("the request fails and nothing is connected").
			ExpectStatus(http.StatusBadGateway).
			ExpectFacts().
			ExpectText(`#repositories-panel [role="alert"]`, "GitHub rejected the token. Check GITHUB_TOKEN.")
	})

	f.Run("repositories page lists connected repositories", func(s *extensions.Scenario) {
		s.Covers("REPO-2.1.1", "REPO-2.1.2").
			Given(
				contract.RepositoryConnected{FullName: "acme/api"},
				contract.RepositoryConnected{FullName: "acme/web"},
				contract.RepositoryConnected{FullName: "acme/old"},
				contract.RepositoryDisconnected{FullName: "acme/old"},
			).
			When(spec.Get("/repositories")).
			Then("connected repositories are listed and disconnected ones are not").
			ExpectStatus(http.StatusOK).
			ExpectFacts().
			ExpectElement(`#repositories li[data-repository="acme/web"]`).
			ExpectElement(`#repositories li[data-repository="acme/api"]`).
			ExpectNoElement(`#repositories li[data-repository="acme/old"]`)
	})

	f.Run("repositories page lists newest connections first", func(s *extensions.Scenario) {
		s.Covers("REPO-2.2.1").
			Given(
				contract.RepositoryConnected{FullName: "acme/api"},
				contract.RepositoryConnected{FullName: "acme/worker"},
				contract.RepositoryConnected{FullName: "acme/web"},
			).
			When(spec.Get("/repositories")).
			Then("repositories are listed as web, worker, api").
			ExpectStatus(http.StatusOK).
			ExpectFacts().
			ExpectElement(`#repositories > li:nth-child(1)[data-repository="acme/web"]`).
			ExpectElement(`#repositories > li:nth-child(2)[data-repository="acme/worker"]`).
			ExpectElement(`#repositories > li:nth-child(3):last-child[data-repository="acme/api"]`)
	})

	f.Run("connect a repository at the top of the list", func(s *extensions.Scenario) {
		s.GivenGitHubSees("acme/web").
			Covers("REPO-1.2.1", "REPO-1.2.2", "REPO-2.2.1").
			Given(
				contract.RepositoryConnected{FullName: "acme/api"},
				contract.RepositoryConnected{FullName: "acme/worker"},
			).
			When(spec.Post("/repositories").WithForm("repo", "acme/web")).
			Then("web is connected above worker and api").
			ExpectStatus(http.StatusOK).
			ExpectFacts(contract.RepositoryConnected{FullName: "acme/web"}).
			ExpectElement(`#repositories > li:nth-child(1)[data-repository="acme/web"]`).
			ExpectElement(`#repositories > li:nth-child(2)[data-repository="acme/worker"]`).
			ExpectElement(`#repositories > li:nth-child(3):last-child[data-repository="acme/api"]`)
	})

	f.Run("reconnect a repository at the top of the list", func(s *extensions.Scenario) {
		s.GivenGitHubSees("acme/web").
			Covers("REPO-2.2.2", "REPO-3.4.1").
			Given(
				contract.RepositoryConnected{FullName: "acme/web"},
				contract.RepositoryConnected{FullName: "acme/api"},
				contract.RepositoryConnected{FullName: "acme/worker"},
				contract.RepositoryDisconnected{FullName: "acme/web"},
			).
			When(spec.Post("/repositories").WithForm("repo", "acme/web")).
			Then("web is reconnected above worker and api without a duplicate row").
			ExpectStatus(http.StatusOK).
			ExpectFacts(contract.RepositoryConnected{FullName: "acme/web"}).
			ExpectElement(`#repositories > li:nth-child(1)[data-repository="acme/web"]`).
			ExpectElement(`#repositories > li:nth-child(2)[data-repository="acme/worker"]`).
			ExpectElement(`#repositories > li:nth-child(3):last-child[data-repository="acme/api"]`)
	})

	for _, name := range []string{"acme/api", " Acme/API "} {
		f.Run("connect an already connected repository without reordering "+strconv.Quote(name), func(s *extensions.Scenario) {
			s.Covers("REPO-1.4.1", "REPO-1.4.2", "REPO-2.2.3").
				Given(
					contract.RepositoryConnected{FullName: "acme/api"},
					contract.RepositoryConnected{FullName: "acme/worker"},
					contract.RepositoryConnected{FullName: "acme/web"},
				).
				When(spec.Post("/repositories").WithForm("repo", name)).
				Then("repositories remain listed as web, worker, api and no fact is recorded").
				ExpectStatus(http.StatusOK).
				ExpectFacts().
				ExpectElement(`#repositories > li:nth-child(1)[data-repository="acme/web"]`).
				ExpectElement(`#repositories > li:nth-child(2)[data-repository="acme/worker"]`).
				ExpectElement(`#repositories > li:nth-child(3):last-child[data-repository="acme/api"]`).
				Expect(s.GitHubNotAsked())
		})
	}

	f.Run("repositories page offers to disconnect each repository", func(s *extensions.Scenario) {
		s.Covers("REPO-3.1.1").
			Given(
				contract.RepositoryConnected{FullName: "acme/api"},
				contract.RepositoryConnected{FullName: "acme/web"},
			).
			When(spec.Get("/repositories")).
			Then("every listed repository has a disconnect button").
			ExpectStatus(http.StatusOK).
			ExpectFacts().
			ExpectElement(`#repositories li[data-repository="acme/api"] button[hx-post="/repositories/acme/api/disconnect"]`).
			ExpectElement(`#repositories li[data-repository="acme/web"] button[hx-post="/repositories/acme/web/disconnect"]`)
	})

	f.Run("disconnect a connected repository", func(s *extensions.Scenario) {
		s.Covers("REPO-3.2.1", "REPO-3.2.2").
			Given(
				contract.RepositoryConnected{FullName: "acme/api"},
				contract.RepositoryConnected{FullName: "acme/web"},
			).
			When(spec.Post("/repositories/acme/api/disconnect")).
			Then("the repository is disconnected and only the other one is listed").
			ExpectStatus(http.StatusOK).
			ExpectFacts(contract.RepositoryDisconnected{FullName: "acme/api"}).
			ExpectNoElement(`#repositories-panel li[data-repository="acme/api"]`).
			ExpectElement(`#repositories-panel li[data-repository="acme/web"]`)
	})

	f.Run("disconnect an already disconnected repository", func(s *extensions.Scenario) {
		s.Covers("REPO-3.3.1").
			Given(
				contract.RepositoryConnected{FullName: "acme/api"},
				contract.RepositoryDisconnected{FullName: "acme/api"},
			).
			When(spec.Post("/repositories/acme/api/disconnect")).
			Then("nothing more is recorded").
			ExpectStatus(http.StatusOK).
			ExpectFacts().
			ExpectElement(`#repositories-panel`).
			ExpectNoElement(`#repositories-panel li[data-repository="acme/api"]`)
	})

	f.Run("reconnect a disconnected repository", func(s *extensions.Scenario) {
		s.GivenGitHubSees("acme/api").
			Covers("REPO-3.4.1").
			Given(
				contract.RepositoryConnected{FullName: "acme/api"},
				contract.RepositoryDisconnected{FullName: "acme/api"},
			).
			When(spec.Post("/repositories").WithForm("repo", "acme/api")).
			Then("the repository is connected again and listed").
			ExpectStatus(http.StatusOK).
			ExpectFacts(contract.RepositoryConnected{FullName: "acme/api"}).
			ExpectElement(`#repositories-panel li[data-repository="acme/api"]`)
	})
}
