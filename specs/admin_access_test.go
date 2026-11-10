package specs

import (
	"net/http"
	"testing"

	"github.com/Andrew-Sem/gc/framework/spec"
	"github.com/Andrew-Sem/gc/specs/contract"
	"github.com/Andrew-Sem/gc/specs/extensions"
	"github.com/Andrew-Sem/gc/ui"
)

const refusalHeader = `Basic realm="gc", charset="UTF-8"`

func TestAdminAccess(t *testing.T) {
	f := extensions.NewFeature(t, "admin-access.md")
	connected := contract.RepositoryConnected{FullName: "acme/api"}

	adminRequests := []struct {
		name    string
		request spec.Request
	}{
		{"the repositories page", spec.Get("/repositories")},
		{"the runs page", spec.Get("/runs")},
		{"the runs panel", spec.Get("/runs/panel")},
		{"the event stream", spec.Get("/runs/events")},
		{"the pull requests page", spec.Get("/pulls")},
		{"the pull requests panel", spec.Get("/pulls/panel")},
		{"the settings page", spec.Get("/repositories/acme/api/settings")},
		{"a static file", spec.Get(ui.HTMXPath)},
		{"a connect", spec.Post("/repositories").WithForm("repo", "acme/web")},
		{"a disconnect", spec.Post("/repositories/acme/api/disconnect")},
		{"a run request", spec.Post("/repositories/acme/api/run")},
		{"a sweep request", spec.Post("/repositories/acme/api/sweep")},
		{"a settings save", spec.Post("/repositories/acme/api/settings").WithForm("pace", "1").WithForm("limit", "100").WithForm("merge", "on")},
	}

	for _, tt := range adminRequests {
		f.Run("refuse "+tt.name+" without a login", func(s *extensions.Scenario) {
			s.Covers("ACCESS-1.1.2").
				Given(connected).
				GivenGitHubSees("acme/web").
				When(extensions.Anonymously(tt.request)).
				Then("the request is refused and nothing happens").
				ExpectStatus(http.StatusUnauthorized).
				ExpectFacts().
				Expect(s.GitHubNotAsked()).
				ExpectHeader("WWW-Authenticate", refusalHeader).
				ExpectBodyContaining("Authentication required")
		})
	}

	f.Run("a settings save with the login", func(s *extensions.Scenario) {
		s.Covers("ACCESS-1.1.1").
			Given(connected).
			When(extensions.WithLogin(spec.Post("/repositories/acme/api/settings").WithForm("pace", "1").WithForm("limit", "100").WithForm("merge", "on"), "spec-operator", extensions.AdminPassword())).
			Then("the settings are saved").
			ExpectStatus(http.StatusOK).
			ExpectFacts(contract.RepositorySettingsChanged{Repository: "acme/api", Pace: 1, Limit: 100, Merge: true, At: extensions.At("2000-01-01 00:00")})
	})

	f.Run("a page with the login", func(s *extensions.Scenario) {
		s.Covers("ACCESS-1.1.1").
			Given(connected).
			When(extensions.WithLogin(spec.Get("/repositories"), "spec-operator", extensions.AdminPassword())).
			Then("the page is shown").
			ExpectStatus(http.StatusOK).
			ExpectFacts().
			ExpectElement(`#repositories li[data-repository="acme/api"]`)
	})

	for _, tt := range []struct{ name, user, password string }{
		{"a wrong password", "spec-operator", "guess"},
		{"a wrong user", "admin", extensions.AdminPassword()},
		{"both wrong", "admin", "admin"},
		{"an empty login", "", ""},
		{"the password with a character more", "spec-operator", extensions.AdminPassword() + "x"},
	} {
		f.Run("refuse "+tt.name, func(s *extensions.Scenario) {
			s.Covers("ACCESS-1.1.3").
				Given(connected).
				When(extensions.WithLogin(spec.Post("/repositories/acme/api/run"), tt.user, tt.password)).
				Then("the request is refused and nothing is requested").
				ExpectStatus(http.StatusUnauthorized).
				ExpectFacts().
				ExpectHeader("WWW-Authenticate", refusalHeader)
		})
	}

	for _, tt := range []struct{ name, value string }{
		{"a bearer token", "Bearer " + extensions.AdminPassword()},
		{"Basic that is not Base64", "Basic not-base64!"},
		{"Basic without a colon", "Basic c3BlYy1vcGVyYXRvcg=="},
	} {
		f.Run("refuse "+tt.name, func(s *extensions.Scenario) {
			s.Covers("ACCESS-1.1.4").
				Given(connected).
				When(extensions.WithAuthorization(spec.Post("/repositories/acme/api/run"), tt.value)).
				Then("the request is refused and nothing is requested").
				ExpectStatus(http.StatusUnauthorized).
				ExpectFacts().
				ExpectHeader("WWW-Authenticate", refusalHeader)
		})
	}

	for _, tt := range []struct {
		name    string
		request spec.Request
	}{
		{"an empty login", extensions.WithLogin(spec.Get("/repositories"), "", "")},
		{"no login", extensions.Anonymously(spec.Get("/repositories"))},
		{"the spec operator's login", spec.Get("/repositories")},
	} {
		f.Run("with no admin login set, refuse "+tt.name, func(s *extensions.Scenario) {
			s.Covers("ACCESS-1.1.5").
				GivenNoAdminLogin().
				When(tt.request).
				Then("the admin stays closed").
				ExpectStatus(http.StatusUnauthorized).
				ExpectFacts().
				ExpectHeader("WWW-Authenticate", refusalHeader)
		})
	}

	f.Run("the webhook needs no login", func(s *extensions.Scenario) {
		s.Covers("ACCESS-1.2.1").
			Given(ourPR...).
			When(extensions.Anonymously(delivery(nil))).
			Then("the signed delivery is accepted and recorded").
			ExpectStatus(http.StatusNoContent).
			ExpectFacts(snapshot(nil))
	})

	f.Run("the webhook with no admin login set", func(s *extensions.Scenario) {
		s.Covers("ACCESS-1.2.1").
			GivenNoAdminLogin().
			Given(ourPR...).
			When(extensions.Anonymously(delivery(nil))).
			Then("the signed delivery is still accepted").
			ExpectStatus(http.StatusNoContent).
			ExpectFacts(snapshot(nil))
	})

	f.Run("the password stays out of a refusal", func(s *extensions.Scenario) {
		s.Covers("ACCESS-2.1.1").
			When(extensions.WithLogin(spec.Get("/repositories"), "admin", extensions.AdminPassword())).
			Then("the refusal does not echo the password").
			ExpectStatus(http.StatusUnauthorized).
			ExpectFacts().
			ExpectBodyWithout(extensions.AdminPassword())
	})

	f.Run("the password stays out of a page", func(s *extensions.Scenario) {
		s.Covers("ACCESS-2.1.1").
			Given(connected).
			When(spec.Get("/repositories/acme/api/settings")).
			Then("the page has no password").
			ExpectStatus(http.StatusOK).
			ExpectFacts().
			ExpectBodyWithout(extensions.AdminPassword())
	})

	f.RunLaunch("the credentials stay out of the journal file and the log", func(s *extensions.Launch) {
		s.GivenGitHubSees("acme/api").
			GivenRequest(extensions.WithLogin(spec.Get("/repositories"), "spec-operator", "guess")).
			Covers("ACCESS-2.1.2").
			When(spec.Post("/repositories").WithForm("repo", "acme/api")).
			Then("the connection is recorded, and neither the file nor the output has the login").
			ExpectStatus(http.StatusOK).
			ExpectJournal(connected).
			ExpectJournalFileWithout(extensions.AdminPassword()).
			ExpectJournalFileWithout("spec-operator").
			ExpectOutputWithout(extensions.AdminPassword()).
			ExpectOutputWithout("spec-operator")
	})
}
