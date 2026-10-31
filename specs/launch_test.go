package specs

import (
	"net/http"
	"testing"

	"github.com/Andrew-Sem/gc/framework/spec"
	"github.com/Andrew-Sem/gc/specs/extensions"
)

func TestLaunch(t *testing.T) {
	f := extensions.NewFeature(t, "launch.md")
	f.RunLaunch("start without a GitHub token", func(s *extensions.Launch) {
		s.GivenUnset("GITHUB_TOKEN").
			Covers("RUN-1.1.1").
			WhenStarted().
			Then("gc refuses to start").
			ExpectExit().
			ExpectOutput("GITHUB_TOKEN is not set")
	})

	f.RunLaunch("start with an empty GitHub token", func(s *extensions.Launch) {
		s.GivenEnv("GITHUB_TOKEN", "").
			Covers("RUN-1.1.1").
			WhenStarted().
			Then("gc refuses to start").
			ExpectExit().
			ExpectOutput("GITHUB_TOKEN is not set")
	})

	f.RunLaunch("start on the address in GC_ADDR", func(s *extensions.Launch) {
		s.Covers("RUN-1.2.1").
			WhenStarted().
			Then("the repositories page is served at GC_ADDR").
			ExpectServing()
	})

	f.RunLaunch("start without an address", func(s *extensions.Launch) {
		s.GivenUnset("GC_ADDR").
			Covers("RUN-1.2.4").
			WhenStarted().
			Then("gc refuses to start").
			ExpectExit().
			ExpectOutput("GC_ADDR is not set")
	})

	f.RunLaunch("start on a malformed address", func(s *extensions.Launch) {
		s.GivenEnv("GC_ADDR", "no-port").
			Covers("RUN-1.2.3").
			WhenStarted().
			Then("gc refuses to start").
			ExpectExit().
			ExpectOutput("cannot listen on no-port")
	})

	f.RunLaunch("start on a taken port", func(s *extensions.Launch) {
		s.GivenAddressTaken().
			Covers("RUN-1.2.3").
			WhenStarted().
			Then("gc refuses to start").
			ExpectExit().
			ExpectOutput("cannot listen on " + s.Address())
	})

	f.RunLaunch("connect through api.github.com without GITHUB_API_URL", func(s *extensions.Launch) {
		s.GivenUnset("GITHUB_API_URL").
			Covers("RUN-1.3.2", "RUN-1.3.3").
			When(spec.Post("/repositories").WithForm("repo", "acme/api")).
			Then("gc asks api.github.com through the proxy, which refuses").
			ExpectStatus(http.StatusBadGateway).
			ExpectProxyAsked("api.github.com:443")
	})

	f.RunLaunch("connect through the proxy in HTTPS_PROXY", func(s *extensions.Launch) {
		s.GivenEnv("GITHUB_API_URL", "https://github.example.test").
			Covers("RUN-1.3.3").
			When(spec.Post("/repositories").WithForm("repo", "acme/api")).
			Then("gc asks the configured API through the proxy, which refuses").
			ExpectStatus(http.StatusBadGateway).
			ExpectProxyAsked("github.example.test:443")
	})

	f.RunLaunch("connect through the GitHub API in GITHUB_API_URL", func(s *extensions.Launch) {
		s.GivenGitHubSees("acme/api").
			Covers("RUN-1.3.1").
			When(spec.Post("/repositories").WithForm("repo", "acme/api")).
			Then("the repository is connected after asking the configured API").
			ExpectStatus(http.StatusOK)
	})

	f.RunLaunch("start without a webhook secret", func(s *extensions.Launch) {
		s.GivenUnset("GC_WEBHOOK_SECRET").
			Covers("RUN-1.4.1").
			WhenStarted().
			Then("gc refuses to start").
			ExpectExit().
			ExpectOutput("GC_WEBHOOK_SECRET is not set")
	})
}
