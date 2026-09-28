package specs

import (
	"net/http"
	"testing"

	"github.com/andrew-semyonov/garbage-collector/internal/domain"
	"github.com/andrew-semyonov/garbage-collector/internal/spec"
)

const (
	repo = "acme/garbage-collector"
	row  = `[data-spec="repository-row"][data-repo="acme/garbage-collector"]`
)

func TestConnectRepository(t *testing.T) {
	feature := spec.Feature(t, "connect-repository.md")

	feature.Scenario("page offers the connect form", func(s *spec.Scenario) {
		s.When(spec.Get("/repositories")).
			Then("the operator sees a form with one owner/name field, sent through htmx").
			Covers("REPO-1").
			ExpectStatus(http.StatusOK).
			ExpectNoFacts().
			ExpectHTML(`form[data-spec="connect-form"][hx-post="/repositories"][hx-target="#repositories"] input[name="repo"]`).
			ExpectHTMLCount(`form[data-spec="connect-form"] input`, 1).
			ExpectHTML(`script[data-spec="htmx"]`)
	})

	feature.Scenario("operator connects an accessible repository", func(s *spec.Scenario) {
		s.GitHubCanAccess(repo)

		s.When(spec.PostForm("/repositories", spec.Fields{"repo": repo})).
			Then("the repository is connected and listed").
			Covers("REPO-1.1").
			ExpectStatus(http.StatusOK).
			ExpectFacts(domain.RepositoryConnected{Repo: repo}).
			ExpectHTML(row).
			ExpectNoHTML(`[data-spec="error"]`)
	})

	feature.Scenario("inaccessible repository is not connected", func(s *spec.Scenario) {
		s.GitHubCannotAccess(repo)

		s.When(spec.PostForm("/repositories", spec.Fields{"repo": repo})).
			Then("the page says there is no access").
			Covers("REPO-1.2").
			ExpectStatus(http.StatusUnprocessableEntity).
			ExpectNoFacts().
			ExpectHTML(`[data-spec="error"][data-error="no-access"]`).
			ExpectNoHTML(row)
	})

	feature.Scenario("reconnecting a connected repository records nothing", func(s *spec.Scenario) {
		s.Given(domain.RepositoryConnected{Repo: repo})

		s.When(spec.PostForm("/repositories", spec.Fields{"repo": repo})).
			Then("nothing changes and GitHub is not asked").
			Covers("REPO-1.3").
			ExpectStatus(http.StatusOK).
			ExpectNoFacts().
			ExpectHTMLCount(row, 1)
	})

	feature.Scenario("malformed name is rejected without asking GitHub", func(s *spec.Scenario) {
		s.When(spec.PostForm("/repositories", spec.Fields{"repo": "garbage collector"})).
			Then("the page asks for owner/name").
			Covers("REPO-1.4").
			ExpectStatus(http.StatusUnprocessableEntity).
			ExpectNoFacts().
			ExpectHTML(`[data-spec="error"][data-error="invalid-name"]`)
	})

	feature.Scenario("GitHub outage does not connect the repository", func(s *spec.Scenario) {
		s.GitHubFails(repo)

		s.When(spec.PostForm("/repositories", spec.Fields{"repo": repo})).
			Then("the page says access could not be checked").
			Covers("REPO-1.5").
			ExpectStatus(http.StatusBadGateway).
			ExpectNoFacts().
			ExpectHTML(`[data-spec="error"][data-error="github-unavailable"]`).
			ExpectNoHTML(row)
	})

	feature.Scenario("page lists connected repositories", func(s *spec.Scenario) {
		s.Given(
			domain.RepositoryConnected{Repo: "acme/one"},
			domain.RepositoryConnected{Repo: "acme/two"},
			domain.RepositoryConnected{Repo: "acme/gone"},
			domain.RepositoryDisconnected{Repo: "acme/gone"},
		)

		s.When(spec.Get("/repositories")).
			Then("only currently connected repositories are listed").
			Covers("REPO-2").
			ExpectStatus(http.StatusOK).
			ExpectNoFacts().
			ExpectHTMLCount(`[data-spec="repository-row"]`, 2).
			ExpectHTML(`[data-spec="repository-row"][data-repo="acme/one"]`).
			ExpectHTML(`[data-spec="repository-row"][data-repo="acme/two"]`)
	})

	feature.Scenario("each listed repository has a disconnect button", func(s *spec.Scenario) {
		s.Given(
			domain.RepositoryConnected{Repo: "acme/one"},
			domain.RepositoryConnected{Repo: "acme/two"},
		)

		s.When(spec.Get("/repositories")).
			Then("the button sends that repository to /repositories/disconnect").
			Covers("REPO-3").
			ExpectStatus(http.StatusOK).
			ExpectNoFacts().
			ExpectHTML(`[data-repo="acme/one"] form[hx-post="/repositories/disconnect"] input[name="repo"][value="acme/one"]`).
			ExpectHTML(`[data-repo="acme/two"] form[hx-post="/repositories/disconnect"] input[name="repo"][value="acme/two"]`)
	})

	feature.Scenario("operator disconnects a repository", func(s *spec.Scenario) {
		s.Given(domain.RepositoryConnected{Repo: repo})

		s.When(spec.PostForm("/repositories/disconnect", spec.Fields{"repo": repo})).
			Then("the repository is disconnected and leaves the list").
			Covers("REPO-3.1").
			ExpectStatus(http.StatusOK).
			ExpectFacts(domain.RepositoryDisconnected{Repo: repo}).
			ExpectNoHTML(row)
	})

	feature.Scenario("disconnecting a repository that is not connected records nothing", func(s *spec.Scenario) {
		s.When(spec.PostForm("/repositories/disconnect", spec.Fields{"repo": repo})).
			Then("nothing is recorded").
			Covers("REPO-3.2").
			ExpectStatus(http.StatusOK).
			ExpectNoFacts()
	})

	feature.Scenario("reconnecting after a disconnect appends to history", func(s *spec.Scenario) {
		s.Given(
			domain.RepositoryConnected{Repo: repo},
			domain.RepositoryDisconnected{Repo: repo},
		)
		s.GitHubCanAccess(repo)

		s.When(spec.PostForm("/repositories", spec.Fields{"repo": repo})).
			Then("a new connection is recorded after the old facts").
			Covers("REPO-3.3").
			ExpectStatus(http.StatusOK).
			ExpectFacts(domain.RepositoryConnected{Repo: repo}).
			ExpectHTML(row)
	})

	// Утечку PAT фреймворк проверяет в каждом сценарии. Здесь худший случай: GitHub падает и
	// возвращает токен в теле ошибки.
	feature.Scenario("PAT stays hidden when GitHub echoes it back", func(s *spec.Scenario) {
		s.GitHubFails(repo)

		s.When(spec.PostForm("/repositories", spec.Fields{"repo": repo})).
			Then("the token stays out of the journal, the page and the logs").
			Covers("REPO-4").
			ExpectStatus(http.StatusBadGateway).
			ExpectNoFacts()
	})
}
