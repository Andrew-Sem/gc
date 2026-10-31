package specs

import (
	"testing"
	"time"

	"github.com/Andrew-Sem/gc/specs/contract"
	"github.com/Andrew-Sem/gc/specs/extensions"
)

func TestOpenPullRequests(t *testing.T) {
	f := extensions.NewFeature(t, "open-pull-requests.md")

	f.Run("no decision right after the start", func(s *extensions.Scenario) {
		s.Covers("OPEN-1.1.3").
			Given(contract.RepositoryConnected{FullName: "acme/api"}).
			WhenRunning("14:20", 30*time.Minute).
			Then("nothing is decided before the next hour").
			ExpectFacts()
	})

	f.Run("no decision when started exactly on the hour", func(s *extensions.Scenario) {
		s.Covers("OPEN-1.1.3").
			Given(contract.RepositoryConnected{FullName: "acme/api"}).
			WhenRunning("15:00", 30*time.Minute).
			Then("nothing is decided before the next hour").
			ExpectFacts()
	})

	f.Run("the first decision comes at the next hour", func(s *extensions.Scenario) {
		s.Covers("OPEN-1.1.5").
			Given(contract.RepositoryConnected{FullName: "acme/api"}).
			WhenRunning("14:20", 41*time.Minute).
			Then("one PR is approved at 15:00").
			ExpectFacts(contract.AdmissionApproved{Repository: "acme/api"})
	})

	f.Run("a decision comes every hour", func(s *extensions.Scenario) {
		s.Covers("OPEN-1.1.5").
			Given(contract.RepositoryConnected{FullName: "acme/api"}).
			WhenRunning("14:20", 2*time.Hour+41*time.Minute).
			Then("a PR is approved at 15:00, 16:00, and 17:00").
			ExpectFacts(
				contract.AdmissionApproved{Repository: "acme/api"},
				contract.AdmissionApproved{Repository: "acme/api"},
				contract.AdmissionApproved{Repository: "acme/api"},
			)
	})

	f.RunLaunch("print the next wake-up at the start", func(s *extensions.Launch) {
		s.Covers("OPEN-1.1.6").
			WhenStarted().
			Then("the operator sees that the timer is running").
			ExpectServing().
			ExpectOutput("next wake-up at")
	})

	f.Run("approve a PR when there is room", func(s *extensions.Scenario) {
		s.Covers("OPEN-1.2.1").
			Given(contract.RepositoryConnected{FullName: "acme/api"}).
			WhenWokenUp().
			Then("a new PR is approved").
			ExpectFacts(contract.AdmissionApproved{Repository: "acme/api"})
	})

	f.Run("approve a PR when one place is left", func(s *extensions.Scenario) {
		s.Covers("OPEN-1.2.1").
			Given(contract.RepositoryConnected{FullName: "acme/api"}).
			GivenOpenPRs("acme/api", 11).
			WhenWokenUp().
			Then("a new PR is approved").
			ExpectFacts(contract.AdmissionApproved{Repository: "acme/api"})
	})

	f.Run("decline a PR when the limit is reached", func(s *extensions.Scenario) {
		s.Covers("OPEN-1.3.1").
			Given(contract.RepositoryConnected{FullName: "acme/api"}).
			GivenOpenPRs("acme/api", 12).
			WhenWokenUp().
			Then("no new PR is approved and the numbers are recorded").
			ExpectFacts(contract.AdmissionDeclined{Repository: "acme/api", Open: 12, Limit: 12})
	})

	f.Run("decline a PR when the limit is exceeded", func(s *extensions.Scenario) {
		s.Covers("OPEN-1.3.1").
			Given(contract.RepositoryConnected{FullName: "acme/api"}).
			GivenOpenPRs("acme/api", 13).
			WhenWokenUp().
			Then("no new PR is approved and the numbers are recorded").
			ExpectFacts(contract.AdmissionDeclined{Repository: "acme/api", Open: 13, Limit: 12})
	})

	f.Run("an earlier approval takes no place", func(s *extensions.Scenario) {
		s.Covers("OPEN-1.2.2").
			Given(contract.RepositoryConnected{FullName: "acme/api"}).
			GivenOpenPRs("acme/api", 11).
			Given(contract.AdmissionApproved{Repository: "acme/api"}, contract.AdmissionApproved{Repository: "acme/api"}).
			WhenWokenUp().
			Then("a new PR is approved again").
			ExpectFacts(contract.AdmissionApproved{Repository: "acme/api"})
	})

	f.Run("decide for each connected repository", func(s *extensions.Scenario) {
		s.Covers("OPEN-1.1.1", "OPEN-1.1.4").
			Given(contract.RepositoryConnected{FullName: "acme/api"}, contract.RepositoryConnected{FullName: "acme/web"}).
			WhenWokenUp().
			Then("each repository gets one decision, the most recently connected first").
			ExpectFacts(contract.AdmissionApproved{Repository: "acme/web"}, contract.AdmissionApproved{Repository: "acme/api"})
	})

	f.Run("each repository is decided on its own", func(s *extensions.Scenario) {
		s.Covers("OPEN-1.1.1").
			Given(contract.RepositoryConnected{FullName: "acme/api"}, contract.RepositoryConnected{FullName: "acme/web"}).
			GivenOpenPRs("acme/web", 12).
			WhenWokenUp().
			Then("the full repository is declined and the other is approved").
			ExpectFacts(
				contract.AdmissionDeclined{Repository: "acme/web", Open: 12, Limit: 12},
				contract.AdmissionApproved{Repository: "acme/api"},
			)
	})

	f.Run("no decision without connected repositories", func(s *extensions.Scenario) {
		s.Covers("OPEN-1.1.2").
			WhenWokenUp().
			Then("nothing is decided").
			ExpectFacts()
	})

	f.Run("no decision for a disconnected repository", func(s *extensions.Scenario) {
		s.Covers("OPEN-1.1.2").
			Given(contract.RepositoryConnected{FullName: "acme/api"}, contract.RepositoryDisconnected{FullName: "acme/api"}).
			WhenWokenUp().
			Then("nothing is decided").
			ExpectFacts()
	})

	f.Run("PRs of another repository do not count", func(s *extensions.Scenario) {
		s.Covers("OPEN-1.4.1").
			Given(contract.RepositoryConnected{FullName: "acme/api"}).
			GivenOpenPRs("acme/web", 12).
			WhenWokenUp().
			Then("a new PR is approved").
			ExpectFacts(contract.AdmissionApproved{Repository: "acme/api"})
	})

	f.Run("a closed PR frees a place", func(s *extensions.Scenario) {
		s.Covers("OPEN-1.4.2").
			Given(contract.RepositoryConnected{FullName: "acme/api"}).
			GivenOpenPRs("acme/api", 12).
			Given(contract.PrStateChanged{Repository: "acme/api", Number: 1, Open: false}).
			WhenWokenUp().
			Then("a new PR is approved").
			ExpectFacts(contract.AdmissionApproved{Repository: "acme/api"})
	})

	f.Run("a PR closed twice frees one place", func(s *extensions.Scenario) {
		s.Covers("OPEN-1.4.2").
			Given(contract.RepositoryConnected{FullName: "acme/api"}).
			GivenOpenPRs("acme/api", 13).
			Given(
				contract.PrStateChanged{Repository: "acme/api", Number: 1, Open: false},
				contract.PrStateChanged{Repository: "acme/api", Number: 1, Open: false},
			).
			WhenWokenUp().
			Then("no new PR is approved and the numbers are recorded").
			ExpectFacts(contract.AdmissionDeclined{Repository: "acme/api", Open: 12, Limit: 12})
	})

	f.Run("closing a PR that is not ours frees no place", func(s *extensions.Scenario) {
		s.Covers("OPEN-1.4.4").
			Given(contract.RepositoryConnected{FullName: "acme/api"}).
			GivenOpenPRs("acme/api", 12).
			Given(contract.PrStateChanged{Repository: "acme/api", Number: 99, Open: false}).
			WhenWokenUp().
			Then("no new PR is approved and the numbers are recorded").
			ExpectFacts(contract.AdmissionDeclined{Repository: "acme/api", Open: 12, Limit: 12})
	})

	f.Run("an open PR that is not ours takes no place", func(s *extensions.Scenario) {
		s.Covers("OPEN-1.4.4").
			Given(contract.RepositoryConnected{FullName: "acme/api"}).
			GivenOpenPRs("acme/api", 11).
			Given(contract.PrStateChanged{Repository: "acme/api", Number: 99, Open: true}).
			WhenWokenUp().
			Then("a new PR is approved").
			ExpectFacts(contract.AdmissionApproved{Repository: "acme/api"})
	})

	f.Run("a reopened PR takes its place again", func(s *extensions.Scenario) {
		s.Covers("OPEN-1.4.2").
			Given(contract.RepositoryConnected{FullName: "acme/api"}).
			GivenOpenPRs("acme/api", 12).
			Given(
				contract.PrStateChanged{Repository: "acme/api", Number: 1, Open: false},
				contract.PrStateChanged{Repository: "acme/api", Number: 1, Open: true},
			).
			WhenWokenUp().
			Then("no new PR is approved and the numbers are recorded").
			ExpectFacts(contract.AdmissionDeclined{Repository: "acme/api", Open: 12, Limit: 12})
	})

	f.Run("a closed PR of another repository frees no place", func(s *extensions.Scenario) {
		s.Covers("OPEN-1.4.1").
			Given(contract.RepositoryConnected{FullName: "acme/api"}).
			GivenOpenPRs("acme/api", 12).
			Given(contract.PrStateChanged{Repository: "acme/web", Number: 1, Open: false}).
			WhenWokenUp().
			Then("no new PR is approved and the numbers are recorded").
			ExpectFacts(contract.AdmissionDeclined{Repository: "acme/api", Open: 12, Limit: 12})
	})

	f.Run("PRs count again after reconnecting", func(s *extensions.Scenario) {
		s.Covers("OPEN-1.4.3").
			Given(contract.RepositoryConnected{FullName: "acme/api"}).
			GivenOpenPRs("acme/api", 12).
			Given(contract.RepositoryDisconnected{FullName: "acme/api"}, contract.RepositoryConnected{FullName: "acme/api"}).
			WhenWokenUp().
			Then("no new PR is approved and the numbers are recorded").
			ExpectFacts(contract.AdmissionDeclined{Repository: "acme/api", Open: 12, Limit: 12})
	})
}
