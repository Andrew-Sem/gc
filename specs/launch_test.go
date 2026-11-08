package specs

import (
	"net/http"
	"testing"

	"github.com/Andrew-Sem/gc/framework/spec"
	"github.com/Andrew-Sem/gc/specs/extensions"
)

func TestLaunch(t *testing.T) {
	f := extensions.NewFeature(t, "launch.md")
	for _, name := range []string{"GITHUB_APP_ID", "GITHUB_APP_PRIVATE_KEY_FILE"} {
		clause := map[string]string{"GITHUB_APP_ID": "RUN-1.1.1", "GITHUB_APP_PRIVATE_KEY_FILE": "RUN-1.1.2"}[name]
		f.RunLaunch("start without "+name, func(s *extensions.Launch) {
			s.GivenUnset(name).
				Covers(clause).
				WhenStarted().
				Then("gc refuses to start").
				ExpectExit().
				ExpectOutput(name + " is not set")
		})

		f.RunLaunch("start with an empty "+name, func(s *extensions.Launch) {
			s.GivenEnv(name, "").
				Covers(clause).
				WhenStarted().
				Then("gc refuses to start").
				ExpectExit().
				ExpectOutput(name + " is not set")
		})
	}

	f.RunLaunch("start with a private key file that is missing", func(s *extensions.Launch) {
		s.GivenEnv("GITHUB_APP_PRIVATE_KEY_FILE", s.PathInMissingFolder()).
			Covers("RUN-1.1.3").
			WhenStarted().
			Then("gc refuses to start").
			ExpectExit().
			ExpectOutput("cannot read the private key at " + s.PathInMissingFolder())
	})

	f.RunLaunch("start with a file that is not a private key", func(s *extensions.Launch) {
		s.GivenEnv("GITHUB_APP_PRIVATE_KEY_FILE", s.JournalPath()).
			GivenJournalFileContents("not a key at all").
			Covers("RUN-1.1.3").
			WhenStarted().
			Then("gc refuses to start and does not print the file").
			ExpectExit().
			ExpectOutput("cannot read the private key at " + s.JournalPath()).
			ExpectOutputWithout("not a key at all")
	})

	f.RunLaunch("start with a private key that is not RSA", func(s *extensions.Launch) {
		s.GivenEnv("GITHUB_APP_PRIVATE_KEY_FILE", s.JournalPath()).
			GivenJournalFileContents(extensions.ECPrivateKey).
			Covers("RUN-1.1.3").
			WhenStarted().
			Then("gc refuses to start and does not print the key").
			ExpectExit().
			ExpectOutput("cannot read the private key at " + s.JournalPath()).
			ExpectOutputWithout(extensions.ECPrivateKeyLine)
	})

	f.RunLaunch("the private key stays out of the output", func(s *extensions.Launch) {
		s.GivenGitHubSees("acme/api").
			Covers("RUN-1.1.3").
			When(spec.Post("/repositories").WithForm("repo", "acme/api")).
			Then("gc connects with the key and never prints it").
			ExpectStatus(http.StatusOK).
			ExpectOutputWithout(s.PrivateKeyText())
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

	f.RunLaunch("start with an empty webhook secret", func(s *extensions.Launch) {
		s.GivenEnv("GC_WEBHOOK_SECRET", "").
			Covers("RUN-1.4.1").
			WhenStarted().
			Then("gc refuses to start").
			ExpectExit().
			ExpectOutput("GC_WEBHOOK_SECRET is not set")
	})

	f.RunLaunch("start without an OpenAI key", func(s *extensions.Launch) {
		s.GivenUnset("OPENAI_API_KEY").
			Covers("RUN-1.5.1").
			WhenStarted().
			Then("gc refuses to start").
			ExpectExit().
			ExpectOutput("OPENAI_API_KEY is not set")
	})

	f.RunLaunch("start with an empty OpenAI key", func(s *extensions.Launch) {
		s.GivenEnv("OPENAI_API_KEY", "").
			Covers("RUN-1.5.1").
			WhenStarted().
			Then("gc refuses to start").
			ExpectExit().
			ExpectOutput("OPENAI_API_KEY is not set")
	})

	f.RunLaunch("start without a journal path", func(s *extensions.Launch) {
		s.GivenUnset("GC_DB").
			Covers("RUN-1.6.1").
			WhenStarted().
			Then("gc refuses to start").
			ExpectExit().
			ExpectOutput("GC_DB is not set")
	})

	f.RunLaunch("start with an empty journal path", func(s *extensions.Launch) {
		s.GivenEnv("GC_DB", "").
			Covers("RUN-1.6.1").
			WhenStarted().
			Then("gc refuses to start").
			ExpectExit().
			ExpectOutput("GC_DB is not set")
	})

	f.RunLaunch("accept a webhook signed with the configured secret", func(s *extensions.Launch) {
		s.Covers("RUN-1.4.2").
			When(extensions.PullRequestDelivery(extensions.PullRequest{
				Action:     "opened",
				Repository: "acme/api",
				Number:     7,
			})).
			Then("the delivery is accepted").
			ExpectStatus(http.StatusNoContent)
	})
}
