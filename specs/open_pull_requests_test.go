package specs

import (
	"net/http"
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
			Then("one PR is approved at 15:00, and the agent changes nothing").
			ExpectFacts(
				contract.AdmissionApproved{Repository: "acme/api"},
				contract.AgentRunFinished{Repository: "acme/api", Outcome: contract.OutcomeNoPR},
			)
	})

	f.Run("a decision comes every hour", func(s *extensions.Scenario) {
		s.Covers("OPEN-1.1.5").
			Given(contract.RepositoryConnected{FullName: "acme/api"}).
			WhenRunning("14:20", 2*time.Hour+41*time.Minute).
			Then("a PR is approved at 15:00, 16:00, and 17:00, and the agent changes nothing").
			ExpectFacts(
				contract.AdmissionApproved{Repository: "acme/api"},
				contract.AgentRunFinished{Repository: "acme/api", Outcome: contract.OutcomeNoPR},
				contract.AdmissionApproved{Repository: "acme/api"},
				contract.AgentRunFinished{Repository: "acme/api", Outcome: contract.OutcomeNoPR},
				contract.AdmissionApproved{Repository: "acme/api"},
				contract.AgentRunFinished{Repository: "acme/api", Outcome: contract.OutcomeNoPR},
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

	f.Run("open a PR when the agent changed code", func(s *extensions.Scenario) {
		s.Covers("OPEN-2.1.1", "OPEN-2.2.1").
			Given(contract.RepositoryConnected{FullName: "acme/api"}).
			GivenAgentChanges("gc/run-1", "Remove dead code", "The helper has no callers.").
			GivenGitHubCreatesPR(42).
			WhenWokenUpFor(time.Minute).
			Then("the PR is created and recorded").
			ExpectFacts(
				contract.AdmissionApproved{Repository: "acme/api"},
				contract.PrCreated{Repository: "acme/api", Number: 42},
				contract.AgentRunFinished{Repository: "acme/api", Outcome: contract.OutcomePR, Number: 42},
			).
			Expect(s.AgentAsked("acme/api")).
			Expect(s.GitHubPRCreated("acme/api", "gc/run-1", "Remove dead code", "The helper has no callers."))
	})

	f.Run("each approved repository gets its own run", func(s *extensions.Scenario) {
		s.Covers("OPEN-2.1.1").
			Given(contract.RepositoryConnected{FullName: "acme/api"}, contract.RepositoryConnected{FullName: "acme/web"}).
			GivenAgentHangs().
			WhenWokenUpFor(time.Minute).
			Then("the agent works on both repositories").
			ExpectFacts(
				contract.AdmissionApproved{Repository: "acme/web"},
				contract.AdmissionApproved{Repository: "acme/api"},
			).
			Expect(s.AgentAsked("acme/api", "acme/web"))
	})

	f.Run("no run after a decline", func(s *extensions.Scenario) {
		s.Covers("OPEN-2.1.2").
			Given(contract.RepositoryConnected{FullName: "acme/api"}).
			GivenOpenPRs("acme/api", 12).
			WhenWokenUpFor(time.Minute).
			Then("the agent is not started").
			ExpectFacts(contract.AdmissionDeclined{Repository: "acme/api", Open: 12, Limit: 12}).
			Expect(s.AgentAsked())
	})

	f.Run("a new run works alongside the previous one", func(s *extensions.Scenario) {
		s.Covers("OPEN-2.1.3").
			Given(contract.RepositoryConnected{FullName: "acme/api"}).
			GivenAgentHangs().
			WhenRunning("14:59", time.Hour+2*time.Minute).
			Then("the second run starts while the first is still working").
			ExpectFacts(
				contract.AdmissionApproved{Repository: "acme/api"},
				contract.AdmissionApproved{Repository: "acme/api"},
			).
			Expect(s.AgentAsked("acme/api", "acme/api"))
	})

	f.Run("a created PR takes a place at the next wake-up", func(s *extensions.Scenario) {
		s.Covers("OPEN-2.2.2").
			Given(contract.RepositoryConnected{FullName: "acme/api"}).
			GivenOpenPRs("acme/api", 11).
			GivenAgentChanges("gc/run-1", "Remove dead code", "The helper has no callers.").
			GivenGitHubCreatesPR(42).
			WhenRunning("14:59", time.Hour+2*time.Minute).
			Then("the 15:00 run opens the twelfth PR, and the 16:00 decision is a decline").
			ExpectFacts(
				contract.AdmissionApproved{Repository: "acme/api"},
				contract.PrCreated{Repository: "acme/api", Number: 42},
				contract.AgentRunFinished{Repository: "acme/api", Outcome: contract.OutcomePR, Number: 42},
				contract.AdmissionDeclined{Repository: "acme/api", Open: 12, Limit: 12},
			)
	})

	f.Run("no PR when the agent changed nothing", func(s *extensions.Scenario) {
		s.Covers("OPEN-2.3.1").
			Given(contract.RepositoryConnected{FullName: "acme/api"}).
			WhenWokenUpFor(time.Minute).
			Then("the run is recorded without a PR").
			ExpectFacts(
				contract.AdmissionApproved{Repository: "acme/api"},
				contract.AgentRunFinished{Repository: "acme/api", Outcome: contract.OutcomeNoPR},
			).
			Expect(s.GitHubNotAsked())
	})

	f.Run("no PR when the agent fails", func(s *extensions.Scenario) {
		s.Covers("OPEN-2.4.1").
			Given(contract.RepositoryConnected{FullName: "acme/api"}).
			GivenAgentFails().
			WhenWokenUpFor(time.Minute).
			Then("the run is recorded as failed").
			ExpectFacts(
				contract.AdmissionApproved{Repository: "acme/api"},
				contract.AgentRunFinished{Repository: "acme/api", Outcome: contract.OutcomeFailed},
			).
			Expect(s.GitHubNotAsked())
	})

	f.Run("no PR when GitHub refuses to create it", func(s *extensions.Scenario) {
		s.Covers("OPEN-2.4.2").
			Given(contract.RepositoryConnected{FullName: "acme/api"}).
			GivenAgentChanges("gc/run-1", "Remove dead code", "The helper has no callers.").
			GivenGitHubAnswers(http.StatusUnprocessableEntity).
			WhenWokenUpFor(time.Minute).
			Then("the run is recorded as failed and no PR is counted").
			ExpectFacts(
				contract.AdmissionApproved{Repository: "acme/api"},
				contract.AgentRunFinished{Repository: "acme/api", Outcome: contract.OutcomeFailed},
			)
	})

	f.Run("stop a run at the limit", func(s *extensions.Scenario) {
		s.Covers("OPEN-2.5.1").
			Given(contract.RepositoryConnected{FullName: "acme/api"}).
			GivenAgentHangs().
			WhenWokenUpFor(2*time.Hour+time.Minute).
			Then("the run is stopped and recorded as timed out").
			ExpectFacts(
				contract.AdmissionApproved{Repository: "acme/api"},
				contract.AgentRunFinished{Repository: "acme/api", Outcome: contract.OutcomeTimedOut},
			).
			Expect(s.GitHubNotAsked())
	})

	f.Run("a run just before the limit is not stopped", func(s *extensions.Scenario) {
		s.Covers("OPEN-2.5.2").
			Given(contract.RepositoryConnected{FullName: "acme/api"}).
			GivenAgentChanges("gc/run-1", "Remove dead code", "The helper has no callers.").
			GivenAgentWorksFor(2*time.Hour-time.Minute).
			GivenGitHubCreatesPR(42).
			WhenWokenUpFor(2*time.Hour+time.Minute).
			Then("the PR is created and recorded").
			ExpectFacts(
				contract.AdmissionApproved{Repository: "acme/api"},
				contract.PrCreated{Repository: "acme/api", Number: 42},
				contract.AgentRunFinished{Repository: "acme/api", Outcome: contract.OutcomePR, Number: 42},
			)
	})
}
