package specs

import (
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/Andrew-Sem/gc/framework/spec"
	"github.com/Andrew-Sem/gc/specs/contract"
	"github.com/Andrew-Sem/gc/specs/extensions"
	"github.com/Andrew-Sem/gc/ui"
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
			ExpectElementCount(`form[hx-post="/repositories"] input`, 1).
			ExpectElement(`#repositories-panel[hx-target="#repositories-panel"][hx-swap="outerHTML"] form`)
	})

	f.Run("repositories page uses the shared interface head", func(s *extensions.Scenario) {
		s.Covers("REPO-1.1.2").
			When(spec.Get("/repositories")).
			Then("htmx and its response rules come from ui").
			ExpectStatus(http.StatusOK).
			ExpectFacts().
			ExpectElement(`meta[name="htmx-config"][content='`+ui.HTMXConfig+`']`).
			ExpectElementCount(`meta[name="htmx-config"]`, 1).
			ExpectElement(`script[src="` + ui.HTMXPath + `"]`)
	})

	f.Run("htmx is served from ui", func(s *extensions.Scenario) {
		s.Covers("REPO-1.1.2").
			When(spec.Get(ui.HTMXPath)).
			Then("the shared htmx file is served").
			ExpectStatus(http.StatusOK).
			ExpectFacts().
			ExpectBodyContaining(`version:"` + ui.HTMXVersion + `"`)
	})

	f.Run("connect a repository the token can see", func(s *extensions.Scenario) {
		s.GivenGitHubSees("acme/api").
			Covers("REPO-1.2.1", "REPO-1.2.2", "REPO-2.1.3", "REPO-2.3.1", "REPO-3.1.1").
			When(spec.Post("/repositories").WithForm("repo", "acme/api")).
			Then("the repository is connected and listed").
			ExpectStatus(http.StatusOK).
			ExpectFacts(contract.RepositoryConnected{FullName: "acme/api"}).
			ExpectElement(extensions.UpdatedBlock).
			ExpectNoElement(extensions.WholePage).
			ExpectText(`#repositories-panel li[data-repository="acme/api"]`, "acme/api Run now Disconnect").
			ExpectElement(`#repositories-panel li[data-repository="acme/api"] button[hx-post="/repositories/acme/api/disconnect"]`).
			ExpectNoElement(`#repositories-panel [role="alert"]`)
	})

	f.Run("connect a repository with every allowed character", func(s *extensions.Scenario) {
		s.GivenGitHubSees("acme-co/api_v2.js").
			Covers("REPO-1.2.1", "REPO-1.5.2", "REPO-2.3.1").
			When(spec.Post("/repositories").WithForm("repo", "acme-co/api_v2.js")).
			Then("the repository is connected under the same name").
			ExpectStatus(http.StatusOK).
			ExpectFacts(contract.RepositoryConnected{FullName: "acme-co/api_v2.js"}).
			ExpectElement(extensions.UpdatedBlock).
			ExpectNoElement(extensions.WholePage).
			Expect(s.GitHubAsked("acme-co/api_v2.js"))
	})

	f.Run("connect a repository the app is not installed on", func(s *extensions.Scenario) {
		s.Covers("REPO-1.3.1", "REPO-1.3.2", "REPO-2.3.1").
			Given(contract.RepositoryConnected{FullName: "acme/web"}).
			When(spec.Post("/repositories").WithForm("repo", "acme/api")).
			Then("the request is rejected and nothing is connected").
			ExpectStatus(http.StatusUnprocessableEntity).
			ExpectFacts().
			ExpectElement(extensions.UpdatedBlock).
			ExpectNoElement(extensions.WholePage).
			ExpectElement(`#repositories-panel li[data-repository="acme/web"]`).
			ExpectText(`#repositories-panel [role="alert"]`, "The app is not installed on this repository")
	})

	f.Run("connect a repository GitHub forbids", func(s *extensions.Scenario) {
		s.GivenGitHubAnswers(http.StatusForbidden, "X-RateLimit-Remaining", "4999").
			Covers("REPO-1.3.1", "REPO-1.3.2", "REPO-2.3.1").
			Given(contract.RepositoryConnected{FullName: "acme/web"}).
			When(spec.Post("/repositories").WithForm("repo", "acme/api")).
			Then("the request is rejected and nothing is connected").
			ExpectStatus(http.StatusUnprocessableEntity).
			ExpectFacts().
			ExpectElement(extensions.UpdatedBlock).
			ExpectNoElement(extensions.WholePage).
			ExpectElement(`#repositories-panel li[data-repository="acme/web"]`).
			ExpectText(`#repositories-panel [role="alert"]`, "The app is not installed on this repository")
	})

	for _, status := range []int{
		http.StatusMovedPermanently,
		http.StatusFound,
		http.StatusTemporaryRedirect,
		http.StatusPermanentRedirect,
	} {
		f.Run("connect a repository GitHub moved with "+strconv.Itoa(status), func(s *extensions.Scenario) {
			s.GivenGitHubMovedRepository(status).
				Covers("REPO-1.12.1", "REPO-1.11.1", "REPO-2.3.1").
				Given(contract.RepositoryConnected{FullName: "acme/web"}).
				When(spec.Post("/repositories").WithForm("repo", "acme/api")).
				Then("the redirect is not followed and nothing is connected").
				ExpectStatus(http.StatusUnprocessableEntity).
				ExpectFacts().
				ExpectElement(extensions.UpdatedBlock).
				ExpectNoElement(extensions.WholePage).
				ExpectElement(`#repositories-panel li[data-repository="acme/web"]`).
				ExpectText(`#repositories-panel [role="alert"]`, "GitHub moved this repository. Enter its current name.").
				Expect(s.GitHubAsked("acme/api"))
		})
	}

	f.Run("connect an already connected repository", func(s *extensions.Scenario) {
		s.GivenGitHubSees("acme/api").
			Covers("REPO-1.4.1", "REPO-1.4.2", "REPO-2.3.1").
			Given(contract.RepositoryConnected{FullName: "acme/api"}).
			When(spec.Post("/repositories").WithForm("repo", "acme/api")).
			Then("nothing more is recorded and GitHub is not asked").
			ExpectStatus(http.StatusOK).
			ExpectFacts().
			ExpectElement(extensions.UpdatedBlock).
			ExpectNoElement(extensions.WholePage).
			ExpectElement(`#repositories-panel li[data-repository="acme/api"]`).
			Expect(s.GitHubNotAsked())
	})

	for _, name := range []string{"Acme/API", " acme/api ", " Acme/API ", "\tacme/api\n", "\nAcme/API\t"} {
		f.Run("connect a normalized repository name "+strconv.Quote(name), func(s *extensions.Scenario) {
			s.GivenGitHubSees("acme/api").
				Covers("REPO-1.2.1", "REPO-1.2.2", "REPO-1.7.1", "REPO-1.7.2", "REPO-2.3.1").
				When(spec.Post("/repositories").WithForm("repo", name)).
				Then("the normalized repository name is recorded and displayed").
				ExpectStatus(http.StatusOK).
				ExpectFacts(contract.RepositoryConnected{FullName: "acme/api"}).
				ExpectElement(extensions.UpdatedBlock).
				ExpectNoElement(extensions.WholePage).
				ExpectText(`#repositories-panel li[data-repository="acme/api"]`, "acme/api Run now Disconnect").
				ExpectNoElement(`#repositories-panel [role="alert"]`)
		})
	}

	for _, name := range []string{"Acme/API", " acme/api ", " Acme/API "} {
		f.Run("connect an already connected normalized repository name "+strconv.Quote(name), func(s *extensions.Scenario) {
			s.Covers("REPO-1.4.1", "REPO-1.4.2", "REPO-1.7.3", "REPO-2.3.1").
				Given(contract.RepositoryConnected{FullName: "acme/api"}).
				When(spec.Post("/repositories").WithForm("repo", name)).
				Then("nothing more is recorded and GitHub is not asked").
				ExpectStatus(http.StatusOK).
				ExpectFacts().
				ExpectElement(extensions.UpdatedBlock).
				ExpectNoElement(extensions.WholePage).
				ExpectElement(`#repositories-panel li[data-repository="acme/api"]`).
				ExpectNoElement(`#repositories-panel [role="alert"]`).
				Expect(s.GitHubNotAsked())
		})
	}

	for _, name := range []string{"acmeapi", "/api", "acme/", "acme/api/extra", "", "ac me/api", "acme/ap i", "acme /api", "acme/ api", "   ",
		"acme/api?x", "acme/a#b", "acme/a%20b", "acme_co/api", "acmé/api", "acme/..",
	} {
		f.Run("reject malformed repository name "+strconv.Quote(name), func(s *extensions.Scenario) {
			s.GivenGitHubSees(name).
				Covers("REPO-1.5.1", "REPO-1.5.2", "REPO-2.3.1").
				Given(contract.RepositoryConnected{FullName: "acme/web"}).
				When(spec.Post("/repositories").WithForm("repo", name)).
				Then("the request is rejected and GitHub is not asked").
				ExpectStatus(http.StatusUnprocessableEntity).
				ExpectFacts().
				ExpectElement(extensions.UpdatedBlock).
				ExpectNoElement(extensions.WholePage).
				ExpectElement(`#repositories-panel li[data-repository="acme/web"]`).
				ExpectText(`#repositories-panel [role="alert"]`, "Enter the repository as owner/name").
				Expect(s.GitHubNotAsked())
		})
	}

	f.Run("connect while GitHub does not answer", func(s *extensions.Scenario) {
		s.GivenGitHubDoesNotAnswer().
			Covers("REPO-1.6.1", "REPO-1.6.3", "REPO-1.11.1", "REPO-2.3.1").
			Given(contract.RepositoryConnected{FullName: "acme/web"}).
			When(spec.Post("/repositories").WithForm("repo", "acme/api")).
			Then("the request fails and nothing is connected").
			ExpectStatus(http.StatusBadGateway).
			ExpectFacts().
			ExpectElement(extensions.UpdatedBlock).
			ExpectNoElement(extensions.WholePage).
			ExpectElement(`#repositories-panel li[data-repository="acme/web"]`).
			ExpectText(`#repositories-panel [role="alert"]`, "Could not check access: GitHub did not respond").
			Expect(s.GitHubAsked("acme/api"))
	})

	f.Run("connect while GitHub is unreachable", func(s *extensions.Scenario) {
		s.GivenGitHubUnreachable().
			Covers("REPO-1.6.1", "REPO-1.6.3", "REPO-1.11.1", "REPO-2.3.1").
			Given(contract.RepositoryConnected{FullName: "acme/web"}).
			When(spec.Post("/repositories").WithForm("repo", "acme/api")).
			Then("the request fails and nothing is connected").
			ExpectStatus(http.StatusBadGateway).
			ExpectFacts().
			ExpectElement(extensions.UpdatedBlock).
			ExpectNoElement(extensions.WholePage).
			ExpectElement(`#repositories-panel li[data-repository="acme/web"]`).
			ExpectText(`#repositories-panel [role="alert"]`, "Could not check access: GitHub did not respond").
			Expect(s.GitHubAsked("acme/api"))
	})

	f.Run("connect while GitHub hangs", func(s *extensions.Scenario) {
		s.GivenGitHubHangs().
			Covers("REPO-1.6.1", "REPO-1.6.2", "REPO-1.11.1", "REPO-2.3.1").
			Given(contract.RepositoryConnected{FullName: "acme/web"}).
			When(spec.Post("/repositories").WithForm("repo", "acme/api")).
			Then("the application stops waiting after 5 seconds and nothing is connected").
			ExpectStatus(http.StatusBadGateway).
			ExpectFacts().
			ExpectElement(extensions.UpdatedBlock).
			ExpectNoElement(extensions.WholePage).
			ExpectElement(`#repositories-panel li[data-repository="acme/web"]`).
			ExpectText(`#repositories-panel [role="alert"]`, "Could not check access: GitHub did not respond").
			Expect(s.GitHubWaited(5 * time.Second)).
			Expect(s.GitHubAsked("acme/api"))
	})

	f.Run("connect while GitHub rejects the token", func(s *extensions.Scenario) {
		s.GivenGitHubRejectsToken().
			Covers("REPO-1.8.1", "REPO-1.8.2", "REPO-1.11.1", "REPO-2.3.1").
			Given(contract.RepositoryConnected{FullName: "acme/web"}).
			When(spec.Post("/repositories").WithForm("repo", "acme/api")).
			Then("the request fails and nothing is connected").
			ExpectStatus(http.StatusBadGateway).
			ExpectFacts().
			ExpectElement(extensions.UpdatedBlock).
			ExpectNoElement(extensions.WholePage).
			ExpectElement(`#repositories-panel li[data-repository="acme/web"]`).
			ExpectText(`#repositories-panel [role="alert"]`, "GitHub rejected the app. Check GITHUB_APP_ID and GITHUB_APP_PRIVATE_KEY_FILE.").
			Expect(s.GitHubAsked("acme/api"))
	})

	for _, status := range []int{
		http.StatusBadRequest,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
	} {
		f.Run("connect while GitHub fails with "+strconv.Itoa(status), func(s *extensions.Scenario) {
			s.GivenGitHubAnswers(status).
				Covers("REPO-1.9.1", "REPO-1.11.1", "REPO-2.3.1").
				Given(contract.RepositoryConnected{FullName: "acme/web"}).
				When(spec.Post("/repositories").WithForm("repo", "acme/api")).
				Then("the request fails and nothing is connected").
				ExpectStatus(http.StatusBadGateway).
				ExpectFacts().
				ExpectElement(extensions.UpdatedBlock).
				ExpectNoElement(extensions.WholePage).
				ExpectElement(`#repositories-panel li[data-repository="acme/web"]`).
				ExpectText(`#repositories-panel [role="alert"]`, "Could not check access: GitHub failed. Try again later.").
				Expect(s.GitHubAsked("acme/api"))
		})
	}

	for _, limit := range []struct {
		name   string
		status int
		header []string
	}{
		{"429", http.StatusTooManyRequests, nil},
		{"403 with no requests remaining", http.StatusForbidden, []string{"X-RateLimit-Remaining", "0"}},
		{"403 with retry-after", http.StatusForbidden, []string{"Retry-After", "60"}},
	} {
		f.Run("connect while GitHub limits requests with "+limit.name, func(s *extensions.Scenario) {
			s.GivenGitHubAnswers(limit.status, limit.header...).
				Covers("REPO-1.10.1", "REPO-1.10.2", "REPO-1.11.1", "REPO-2.3.1").
				Given(contract.RepositoryConnected{FullName: "acme/web"}).
				When(spec.Post("/repositories").WithForm("repo", "acme/api")).
				Then("the request fails and nothing is connected").
				ExpectStatus(http.StatusBadGateway).
				ExpectFacts().
				ExpectElement(extensions.UpdatedBlock).
				ExpectNoElement(extensions.WholePage).
				ExpectElement(`#repositories-panel li[data-repository="acme/web"]`).
				ExpectText(`#repositories-panel [role="alert"]`, "GitHub rate limit reached. Try again later.").
				Expect(s.GitHubAsked("acme/api"))
		})
	}

	f.Run("repositories page lists connected repositories", func(s *extensions.Scenario) {
		s.Covers("REPO-2.1.1", "REPO-2.1.2", "REPO-2.1.3").
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
			ExpectElementCount(`#repositories li`, 2).
			ExpectText(`#repositories li[data-repository="acme/web"]`, "acme/web Run now Disconnect").
			ExpectText(`#repositories li[data-repository="acme/api"]`, "acme/api Run now Disconnect").
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
			Covers("REPO-1.2.1", "REPO-1.2.2", "REPO-2.2.1", "REPO-2.3.1").
			Given(
				contract.RepositoryConnected{FullName: "acme/api"},
				contract.RepositoryConnected{FullName: "acme/worker"},
			).
			When(spec.Post("/repositories").WithForm("repo", "acme/web")).
			Then("web is connected above worker and api").
			ExpectStatus(http.StatusOK).
			ExpectFacts(contract.RepositoryConnected{FullName: "acme/web"}).
			ExpectElement(extensions.UpdatedBlock).
			ExpectNoElement(extensions.WholePage).
			ExpectElement(`#repositories > li:nth-child(1)[data-repository="acme/web"]`).
			ExpectElement(`#repositories > li:nth-child(2)[data-repository="acme/worker"]`).
			ExpectElement(`#repositories > li:nth-child(3):last-child[data-repository="acme/api"]`)
	})

	f.Run("reconnect a repository at the top of the list", func(s *extensions.Scenario) {
		s.GivenGitHubSees("acme/web").
			Covers("REPO-2.2.2", "REPO-3.4.1", "REPO-2.3.1").
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
			ExpectElement(extensions.UpdatedBlock).
			ExpectNoElement(extensions.WholePage).
			ExpectElement(`#repositories > li:nth-child(1)[data-repository="acme/web"]`).
			ExpectElement(`#repositories > li:nth-child(2)[data-repository="acme/worker"]`).
			ExpectElement(`#repositories > li:nth-child(3):last-child[data-repository="acme/api"]`)
	})

	for _, name := range []string{"acme/api", " Acme/API "} {
		f.Run("connect an already connected repository without reordering "+strconv.Quote(name), func(s *extensions.Scenario) {
			s.Covers("REPO-1.4.1", "REPO-1.4.2", "REPO-2.2.3", "REPO-2.3.1").
				Given(
					contract.RepositoryConnected{FullName: "acme/api"},
					contract.RepositoryConnected{FullName: "acme/worker"},
					contract.RepositoryConnected{FullName: "acme/web"},
				).
				When(spec.Post("/repositories").WithForm("repo", name)).
				Then("repositories remain listed as web, worker, api and no fact is recorded").
				ExpectStatus(http.StatusOK).
				ExpectFacts().
				ExpectElement(extensions.UpdatedBlock).
				ExpectNoElement(extensions.WholePage).
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
		s.Covers("REPO-3.2.1", "REPO-3.2.2", "REPO-2.3.1").
			Given(
				contract.RepositoryConnected{FullName: "acme/api"},
				contract.RepositoryConnected{FullName: "acme/web"},
			).
			When(spec.Post("/repositories/acme/api/disconnect")).
			Then("the repository is disconnected and only the other one is listed").
			ExpectStatus(http.StatusOK).
			ExpectFacts(contract.RepositoryDisconnected{FullName: "acme/api"}).
			ExpectElement(extensions.UpdatedBlock).
			ExpectNoElement(extensions.WholePage).
			ExpectNoElement(`#repositories-panel li[data-repository="acme/api"]`).
			ExpectElement(`#repositories-panel li[data-repository="acme/web"]`)
	})

	f.Run("disconnect a repository keeping the order of the rest", func(s *extensions.Scenario) {
		s.Covers("REPO-2.2.1", "REPO-2.3.1", "REPO-3.2.1", "REPO-3.2.2").
			Given(
				contract.RepositoryConnected{FullName: "acme/api"},
				contract.RepositoryConnected{FullName: "acme/worker"},
				contract.RepositoryConnected{FullName: "acme/web"},
			).
			When(spec.Post("/repositories/acme/worker/disconnect")).
			Then("web and api stay listed in that order").
			ExpectStatus(http.StatusOK).
			ExpectFacts(contract.RepositoryDisconnected{FullName: "acme/worker"}).
			ExpectElement(extensions.UpdatedBlock).
			ExpectNoElement(extensions.WholePage).
			ExpectElement(`#repositories > li:nth-child(1)[data-repository="acme/web"]`).
			ExpectElement(`#repositories > li:nth-child(2):last-child[data-repository="acme/api"]`)
	})

	f.Run("disconnect a repository by another casing", func(s *extensions.Scenario) {
		s.Covers("REPO-3.2.1", "REPO-3.2.3", "REPO-2.3.1").
			Given(contract.RepositoryConnected{FullName: "acme/api"}).
			When(spec.Post("/repositories/Acme/API/disconnect")).
			Then("the normalized repository is disconnected").
			ExpectStatus(http.StatusOK).
			ExpectFacts(contract.RepositoryDisconnected{FullName: "acme/api"}).
			ExpectElement(extensions.UpdatedBlock).
			ExpectNoElement(extensions.WholePage).
			ExpectNoElement(`#repositories-panel li[data-repository="acme/api"]`)
	})

	f.Run("disconnect an already disconnected repository", func(s *extensions.Scenario) {
		s.Covers("REPO-3.3.1", "REPO-2.3.1").
			Given(
				contract.RepositoryConnected{FullName: "acme/api"},
				contract.RepositoryDisconnected{FullName: "acme/api"},
			).
			When(spec.Post("/repositories/acme/api/disconnect")).
			Then("nothing more is recorded").
			ExpectStatus(http.StatusOK).
			ExpectFacts().
			ExpectElement(extensions.UpdatedBlock).
			ExpectNoElement(extensions.WholePage).
			ExpectElement(`#repositories-panel`).
			ExpectNoElement(`#repositories-panel li[data-repository="acme/api"]`)
	})

	f.Run("reconnect a disconnected repository", func(s *extensions.Scenario) {
		s.GivenGitHubSees("acme/api").
			Covers("REPO-3.4.1", "REPO-2.3.1").
			Given(
				contract.RepositoryConnected{FullName: "acme/api"},
				contract.RepositoryDisconnected{FullName: "acme/api"},
			).
			When(spec.Post("/repositories").WithForm("repo", "acme/api")).
			Then("the repository is connected again and listed").
			ExpectStatus(http.StatusOK).
			ExpectFacts(contract.RepositoryConnected{FullName: "acme/api"}).
			ExpectElement(extensions.UpdatedBlock).
			ExpectNoElement(extensions.WholePage).
			ExpectElement(`#repositories-panel li[data-repository="acme/api"]`)
	})

	for _, answer := range []struct {
		github, page int
	}{
		{http.StatusUnauthorized, http.StatusBadGateway},
		{http.StatusForbidden, http.StatusUnprocessableEntity},
		{http.StatusNotFound, http.StatusUnprocessableEntity},
		{http.StatusTooManyRequests, http.StatusBadGateway},
		{http.StatusInternalServerError, http.StatusBadGateway},
	} {
		f.Run("connect while GitHub echoes the token with "+strconv.Itoa(answer.github), func(s *extensions.Scenario) {
			s.GivenGitHubEchoesToken().
				GivenGitHubAnswers(answer.github).
				Covers("REPO-4.1.1", "REPO-4.1.2", "REPO-2.3.1").
				Given(contract.RepositoryConnected{FullName: "acme/web"}).
				When(spec.Post("/repositories").WithForm("repo", "acme/api")).
				Then("the token is neither recorded nor shown").
				ExpectStatus(answer.page).
				ExpectFacts().
				ExpectElement(extensions.UpdatedBlock).
				ExpectNoElement(extensions.WholePage).
				ExpectElement(`#repositories-panel li[data-repository="acme/web"]`).
				ExpectBodyWithout(s.GitHubToken())
		})

		f.RunLaunch("keep the token out of the output when GitHub echoes it with "+strconv.Itoa(answer.github), func(s *extensions.Launch) {
			s.GivenGitHubEchoesToken().
				GivenGitHubAnswers(answer.github).
				Covers("REPO-4.1.3").
				When(spec.Post("/repositories").WithForm("repo", "acme/api")).
				Then("the token does not reach the output").
				ExpectStatus(answer.page).
				ExpectOutputWithout(s.GitHubToken())
		})
	}

	f.RunLaunch("keep the token out of the output when GitHub quotes it in a malformed answer", func(s *extensions.Launch) {
		s.GivenGitHubQuotesTokenInMalformedAnswer().
			Covers("REPO-4.1.3").
			When(spec.Post("/repositories").WithForm("repo", "acme/api")).
			Then("the token does not reach the output").
			ExpectStatus(http.StatusBadGateway).
			ExpectOutputWithout(s.GitHubToken())
	})

	f.RunLaunch("a broken reused connection does not repeat the question to GitHub", func(s *extensions.Launch) {
		s.GivenGitHubSees("acme/api").
			GivenGitHubDropsConnectionFor("acme/web").
			GivenRequest(spec.Post("/repositories").WithForm("repo", "acme/api")).
			Covers("REPO-1.11.1").
			When(spec.Post("/repositories").WithForm("repo", "acme/web")).
			Then("GitHub is asked once about each").
			ExpectStatus(http.StatusBadGateway).
			ExpectGitHubAsked("acme/api", "acme/web")
	})

	for _, site := range []string{"cross-site", "same-site"} {
		f.Run("refuse a connect from another site: "+site, func(s *extensions.Scenario) {
			s.GivenGitHubSees("attacker/evil").
				Covers("REPO-5.1.1").
				When(spec.Post("/repositories").WithForm("repo", "attacker/evil").WithHeader("Sec-Fetch-Site", site)).
				Then("the request is rejected and nothing is connected").
				ExpectStatus(http.StatusForbidden).
				ExpectFacts().
				Expect(s.GitHubNotAsked())
		})
	}

	f.Run("refuse a disconnect from another site", func(s *extensions.Scenario) {
		s.Given(contract.RepositoryConnected{FullName: "acme/api"}).
			Covers("REPO-5.1.1").
			When(spec.Post("/repositories/acme/api/disconnect").WithHeader("Sec-Fetch-Site", "cross-site")).
			Then("the request is rejected and the repository stays connected").
			ExpectStatus(http.StatusForbidden).
			ExpectFacts()
	})

	f.Run("refuse a connect with an Origin of another host", func(s *extensions.Scenario) {
		s.GivenGitHubSees("attacker/evil").
			Covers("REPO-5.1.2").
			When(spec.Post("/repositories").WithForm("repo", "attacker/evil").WithHeader("Origin", "https://evil.example")).
			Then("the request is rejected and nothing is connected").
			ExpectStatus(http.StatusForbidden).
			ExpectFacts().
			Expect(s.GitHubNotAsked())
	})

	f.Run("refuse a disconnect with an Origin of another host", func(s *extensions.Scenario) {
		s.Given(contract.RepositoryConnected{FullName: "acme/api"}).
			Covers("REPO-5.1.2").
			When(spec.Post("/repositories/acme/api/disconnect").WithHeader("Origin", "https://evil.example")).
			Then("the request is rejected and the repository stays connected").
			ExpectStatus(http.StatusForbidden).
			ExpectFacts()
	})

	for _, tt := range []struct {
		name    string
		request spec.Request
	}{
		{"the same site", spec.Post("/repositories").WithForm("repo", "acme/api").WithHeader("Sec-Fetch-Site", "same-origin")},
		{"the browser itself", spec.Post("/repositories").WithForm("repo", "acme/api").WithHeader("Sec-Fetch-Site", "none")},
		{"an Origin of the same host", spec.Post("/repositories").WithForm("repo", "acme/api").WithHeader("Origin", "http://example.com")},
		{"no browser headers", spec.Post("/repositories").WithForm("repo", "acme/api")},
	} {
		f.Run("accept a connect from "+tt.name, func(s *extensions.Scenario) {
			s.GivenGitHubSees("acme/api").
				Covers("REPO-5.1.3").
				When(tt.request).
				Then("the repository is connected").
				ExpectStatus(http.StatusOK).
				ExpectFacts(contract.RepositoryConnected{FullName: "acme/api"})
		})
	}
}
