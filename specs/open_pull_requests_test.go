package specs

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/Andrew-Sem/gc/framework/spec"
	"github.com/Andrew-Sem/gc/specs/contract"
	"github.com/Andrew-Sem/gc/specs/extensions"
)

func TestOpenPullRequests(t *testing.T) {
	at := func(clock string) time.Time { return extensions.At("2000-01-01 " + clock) }
	f := extensions.NewFeature(t, "open-pull-requests.md")

	lateSweep := func(s *extensions.Scenario, pace, pulls int) *extensions.Scenario {
		s.Given(
			contract.RepositoryConnected{FullName: "acme/api"},
			contract.RepositorySettingsChanged{Repository: "acme/api", Pace: pace, Limit: pulls, Merge: true, At: at("06:00")},
		)
		return givenHangingPulls(s, "acme/api", pulls)
	}
	declined := func(pulls int) contract.Fact {
		return contract.AdmissionDeclined{Repository: "acme/api", Open: pulls, Limit: pulls}
	}

	f.Run("a decision after a long sweep follows its planned hour", func(s *extensions.Scenario) {
		lateSweep(s.Covers("OPEN-1.1.7"), 2, 61).
			WhenRunning("05:59", 75*time.Minute).
			Then("the 06:00 decision, made at 07:01, finds the repository due, and the late 07:00 one does not").
			ExpectFacts(append(hangingSweep("acme/api", numbersUpTo(61)), declined(61))...)
	})

	f.Run("a wake-up a minute late still happens", func(s *extensions.Scenario) {
		lateSweep(s.Covers("OPEN-1.1.8"), 1, 61).
			WhenRunning("05:59", 75*time.Minute).
			Then("both the 06:00 and the 07:00 decisions are made at 07:01").
			ExpectFacts(append(hangingSweep("acme/api", numbersUpTo(61)), declined(61), declined(61))...)
	})

	f.Run("a wake-up more than an hour late is skipped", func(s *extensions.Scenario) {
		lateSweep(s.Covers("OPEN-1.1.8"), 1, 121).
			WhenRunning("05:59", 2*time.Hour+15*time.Minute).
			Then("after a sweep until 08:01, the 06:00 and 08:00 decisions are made, and 07:00 is skipped").
			ExpectFacts(append(hangingSweep("acme/api", numbersUpTo(121)), declined(121), declined(121))...)
	})

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
				contract.AgentRunStarted{Run: 1, Repository: "acme/api", At: at("15:00")},
				contract.AgentRunFinished{Run: 1, Repository: "acme/api", Outcome: contract.OutcomeNoPR, At: at("15:00")},
			)
	})

	f.Run("a decision comes every hour", func(s *extensions.Scenario) {
		s.Covers("OPEN-1.1.5").
			Given(contract.RepositoryConnected{FullName: "acme/api"}).
			WhenRunning("14:20", 2*time.Hour+41*time.Minute).
			Then("a PR is approved at 15:00, 16:00, and 17:00, and the agent changes nothing").
			ExpectFacts(
				contract.AdmissionApproved{Repository: "acme/api"},
				contract.AgentRunStarted{Run: 1, Repository: "acme/api", At: at("15:00")},
				contract.AgentRunFinished{Run: 1, Repository: "acme/api", Outcome: contract.OutcomeNoPR, At: at("15:00")},
				contract.AdmissionApproved{Repository: "acme/api"},
				contract.AgentRunStarted{Run: 2, Repository: "acme/api", At: at("16:00")},
				contract.AgentRunFinished{Run: 2, Repository: "acme/api", Outcome: contract.OutcomeNoPR, At: at("16:00")},
				contract.AdmissionApproved{Repository: "acme/api"},
				contract.AgentRunStarted{Run: 3, Repository: "acme/api", At: at("17:00")},
				contract.AgentRunFinished{Run: 3, Repository: "acme/api", Outcome: contract.OutcomeNoPR, At: at("17:00")},
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
		s.Covers("OPEN-2.1.1", "OPEN-2.1.4", "OPEN-2.2.1", "OPEN-2.6.1").
			Given(contract.RepositoryConnected{FullName: "acme/api"}).
			GivenAgentChanges("gc/run-1", "Remove dead code", "The helper has no callers.").
			GivenGitHubCreatesPR(42).
			WhenWokenUpFor(time.Minute).
			Then("the PR is created and recorded").
			ExpectFacts(
				contract.AdmissionApproved{Repository: "acme/api"},
				contract.AgentRunStarted{Run: 1, Repository: "acme/api", At: at("00:00")},
				contract.PrCreated{Repository: "acme/api", Number: 42},
				contract.AgentRunFinished{Run: 1, Repository: "acme/api", Outcome: contract.OutcomePR, Number: 42, At: at("00:00")},
			).
			Expect(s.AgentAsked("acme/api")).
			Expect(s.GitHubPRCreated("acme/api", "gc/run-1", "Remove dead code", "The helper has no callers."))
	})

	f.Run("a new PR is reconciled before the run ends", func(s *extensions.Scenario) {
		pull := extensions.OpenPullOnGitHub()
		run := contract.CheckRun{ID: 9, Name: "test", AppID: 15368, HeadSHA: "abc123", Status: "queued", StartedAt: at("00:00")}
		s.Covers("OPEN-2.2.4").
			Given(contract.RepositoryConnected{FullName: "acme/api"}).
			GivenAgentChanges("gc/run-1", "Remove dead code", "The helper has no callers.").
			GivenGitHubCreatesPR(42).
			GivenPullOnGitHub("acme/api", 42, pull).
			GivenCheckRunsOnGitHub("acme/api", "abc123", run).
			WhenWokenUpFor(time.Minute).
			Then("the PR's state and checks are recorded between PrCreated and the end of the run").
			ExpectFacts(
				contract.AdmissionApproved{Repository: "acme/api"},
				contract.AgentRunStarted{Run: 1, Repository: "acme/api", At: at("00:00")},
				contract.PrCreated{Repository: "acme/api", Number: 42},
				pull.Snapshot("acme/api", 42),
				contract.CheckRunStateChanged{Repository: "acme/api", Number: 42, CheckRun: run},
				contract.RequiredChecksObserved{Repository: "acme/api", Number: 42, HeadSHA: "abc123", Required: []contract.RequiredCheck{{Name: "test"}}},
				contract.AgentRunFinished{Run: 1, Repository: "acme/api", Outcome: contract.OutcomePR, Number: 42, At: at("00:00")},
			)
	})

	f.Run("a new PR that cannot be read is still a PR", func(s *extensions.Scenario) {
		s.Covers("OPEN-2.2.4").
			Given(contract.RepositoryConnected{FullName: "acme/api"}).
			GivenAgentChanges("gc/run-1", "Remove dead code", "The helper has no callers.").
			GivenGitHubCreatesPR(42).
			WhenWokenUpFor(time.Minute).
			Then("nothing more is recorded, and the run ends with the PR").
			ExpectFacts(
				contract.AdmissionApproved{Repository: "acme/api"},
				contract.AgentRunStarted{Run: 1, Repository: "acme/api", At: at("00:00")},
				contract.PrCreated{Repository: "acme/api", Number: 42},
				contract.AgentRunFinished{Run: 1, Repository: "acme/api", Outcome: contract.OutcomePR, Number: 42, At: at("00:00")},
			)
	})

	f.Run("each approved repository gets its own run", func(s *extensions.Scenario) {
		s.Covers("OPEN-2.1.1", "OPEN-2.1.5").
			Given(contract.RepositoryConnected{FullName: "acme/api"}, contract.RepositoryConnected{FullName: "acme/web"}).
			GivenAgentHangs().
			WhenWokenUpFor(time.Minute).
			Then("the agent works on both repositories").
			ExpectFacts(
				contract.AdmissionApproved{Repository: "acme/web"},
				contract.AdmissionApproved{Repository: "acme/api"},
				contract.AgentRunStarted{Run: 1, Repository: "acme/web", At: at("00:00")},
				contract.AgentRunStarted{Run: 2, Repository: "acme/api", At: at("00:00")},
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

	f.Run("runs of different repositories work alongside each other", func(s *extensions.Scenario) {
		s.Covers("OPEN-2.1.3", "OPEN-2.1.4", "OPEN-2.1.5").
			Given(contract.RepositoryConnected{FullName: "acme/api"}, contract.RepositoryConnected{FullName: "acme/web"}).
			GivenAgentHangs().
			WhenRunning("14:59", 2*time.Minute).
			Then("both runs start at 15:00, numbered in order").
			ExpectFacts(
				contract.AdmissionApproved{Repository: "acme/web"},
				contract.AdmissionApproved{Repository: "acme/api"},
				contract.AgentRunStarted{Run: 1, Repository: "acme/web", At: at("15:00")},
				contract.AgentRunStarted{Run: 2, Repository: "acme/api", At: at("15:00")},
			).
			Expect(s.AgentAsked("acme/api", "acme/web"))
	})

	f.Run("a working run declines the next wake-up", func(s *extensions.Scenario) {
		s.Covers("OPEN-1.5.1").
			Given(contract.RepositoryConnected{FullName: "acme/api"}).
			GivenAgentHangs().
			WhenRunning("14:59", time.Hour+2*time.Minute).
			Then("the 16:00 decision is a decline naming the working run, and no second run starts").
			ExpectFacts(
				contract.AdmissionApproved{Repository: "acme/api"},
				contract.AgentRunStarted{Run: 1, Repository: "acme/api", At: at("15:00")},
				contract.AdmissionDeclined{Repository: "acme/api", Running: 1},
			).
			Expect(s.AgentAsked("acme/api"))
	})

	f.Run("a working run declines a run request", func(s *extensions.Scenario) {
		s.Covers("OPEN-1.5.1").
			Given(contract.RepositoryConnected{FullName: "acme/api"}).
			GivenAgentHangs().
			GivenRunRequestedAt("acme/api", "15:30").
			GivenRunRequestedAt("acme/api", "15:31").
			WhenRunning("14:59", 33*time.Minute).
			Then("each request is declined while the 15:00 run works").
			ExpectFacts(
				contract.AdmissionApproved{Repository: "acme/api"},
				contract.AgentRunStarted{Run: 1, Repository: "acme/api", At: at("15:00")},
				contract.RunRequested{Repository: "acme/api"},
				contract.AdmissionDeclined{Repository: "acme/api", Running: 1},
				contract.RunRequested{Repository: "acme/api"},
				contract.AdmissionDeclined{Repository: "acme/api", Running: 1},
			).
			Expect(s.AgentAsked("acme/api"))
	})

	f.Run("a double run request starts one run", func(s *extensions.Scenario) {
		s.Covers("OPEN-1.5.1").
			Given(contract.RepositoryConnected{FullName: "acme/api"}).
			GivenAgentHangs().
			GivenRunRequestedAt("acme/api", "15:30:00").
			GivenRunRequestedAt("acme/api", "15:30:01").
			WhenRunning("15:29", 2*time.Minute).
			Then("the first request starts a run, and the second is declined").
			ExpectFacts(
				contract.RunRequested{Repository: "acme/api"},
				contract.AdmissionApproved{Repository: "acme/api"},
				contract.AgentRunStarted{Run: 1, Repository: "acme/api", At: at("15:30")},
				contract.RunRequested{Repository: "acme/api"},
				contract.AdmissionDeclined{Repository: "acme/api", Running: 1},
			).
			Expect(s.AgentAsked("acme/api"))
	})

	f.Run("a working run declines even without room", func(s *extensions.Scenario) {
		s.Covers("OPEN-1.5.1").
			Given(contract.RepositoryConnected{FullName: "acme/api"}).
			GivenOpenPRs("acme/api", 12).
			Given(contract.AgentRunStarted{Run: 4, Repository: "acme/api", At: at("00:00")}).
			WhenWokenUpFor(time.Minute).
			Then("the decline names the working run, not the open PRs").
			ExpectFacts(contract.AdmissionDeclined{Repository: "acme/api", Running: 4})
	})

	f.Run("a working run of another repository declines nothing", func(s *extensions.Scenario) {
		s.Covers("OPEN-1.5.1").
			Given(contract.RepositoryConnected{FullName: "acme/api"}).
			Given(contract.AgentRunStarted{Run: 1, Repository: "acme/web", At: at("00:00")}).
			WhenWokenUpFor(time.Minute).
			Then("a run of this repository starts too").
			ExpectFacts(
				contract.AdmissionApproved{Repository: "acme/api"},
				contract.AgentRunStarted{Run: 2, Repository: "acme/api", At: at("00:00")},
				contract.AgentRunFinished{Run: 2, Repository: "acme/api", Outcome: contract.OutcomeNoPR, At: at("00:00")},
			)
	})

	for _, outcome := range []contract.Outcome{contract.OutcomePR, contract.OutcomeNoPR, contract.OutcomeFailed, contract.OutcomeTimedOut, contract.OutcomeInterrupted} {
		f.Run("a run that finished "+string(outcome)+" frees the next decision", func(s *extensions.Scenario) {
			s.Covers("OPEN-1.5.2").
				Given(contract.RepositoryConnected{FullName: "acme/api"}).
				Given(
					contract.AgentRunStarted{Run: 1, Repository: "acme/api", At: at("00:00")},
					contract.AgentRunFinished{Run: 1, Repository: "acme/api", Outcome: outcome, At: at("00:30")},
				).
				WhenWokenUpFor(time.Minute).
				Then("a new run starts").
				ExpectFacts(
					contract.AdmissionApproved{Repository: "acme/api"},
					contract.AgentRunStarted{Run: 2, Repository: "acme/api", At: at("00:00")},
					contract.AgentRunFinished{Run: 2, Repository: "acme/api", Outcome: contract.OutcomeNoPR, At: at("00:00")},
				)
		})
	}

	f.Run("a finished run frees the next wake-up", func(s *extensions.Scenario) {
		s.Covers("OPEN-1.5.2").
			Given(contract.RepositoryConnected{FullName: "acme/api"}).
			GivenAgentWorksFor(30*time.Minute).
			WhenRunning("14:59", time.Hour+2*time.Minute).
			Then("the 16:00 decision starts a new run").
			ExpectFacts(
				contract.AdmissionApproved{Repository: "acme/api"},
				contract.AgentRunStarted{Run: 1, Repository: "acme/api", At: at("15:00")},
				contract.AgentRunFinished{Run: 1, Repository: "acme/api", Outcome: contract.OutcomeNoPR, At: at("15:30")},
				contract.AdmissionApproved{Repository: "acme/api"},
				contract.AgentRunStarted{Run: 2, Repository: "acme/api", At: at("16:00")},
			)
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
				contract.AgentRunStarted{Run: 1, Repository: "acme/api", At: at("15:00")},
				contract.PrCreated{Repository: "acme/api", Number: 42},
				contract.AgentRunFinished{Run: 1, Repository: "acme/api", Outcome: contract.OutcomePR, Number: 42, At: at("15:00")},
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
				contract.AgentRunStarted{Run: 1, Repository: "acme/api", At: at("00:00")},
				contract.AgentRunFinished{Run: 1, Repository: "acme/api", Outcome: contract.OutcomeNoPR, At: at("00:00")},
			).
			Expect(s.GitHubNotAsked())
	})

	f.Run("a run without a PR keeps the agent's explanation", func(s *extensions.Scenario) {
		s.Covers("OPEN-2.3.1", "OPEN-2.3.2").
			Given(contract.RepositoryConnected{FullName: "acme/api"}).
			GivenAgentExplains("## Finding\n\nNothing safe to fix.").
			WhenWokenUpFor(time.Minute).
			Then("the end of the run carries the explanation").
			ExpectFacts(
				contract.AdmissionApproved{Repository: "acme/api"},
				contract.AgentRunStarted{Run: 1, Repository: "acme/api", At: at("00:00")},
				contract.AgentRunFinished{Run: 1, Repository: "acme/api", Outcome: contract.OutcomeNoPR, Explanation: "## Finding\n\nNothing safe to fix.", At: at("00:00")},
			)
	})

	f.Run("a run with a PR carries no explanation", func(s *extensions.Scenario) {
		s.Covers("OPEN-2.3.2").
			Given(contract.RepositoryConnected{FullName: "acme/api"}).
			GivenAgentChanges("gc/run-1", "Remove dead code", "The helper has no callers.").
			GivenAgentExplains("## Finding\n\nStray text.").
			GivenGitHubCreatesPR(42).
			WhenWokenUpFor(time.Minute).
			Then("the end of the run has the PR and no explanation").
			ExpectFacts(
				contract.AdmissionApproved{Repository: "acme/api"},
				contract.AgentRunStarted{Run: 1, Repository: "acme/api", At: at("00:00")},
				contract.PrCreated{Repository: "acme/api", Number: 42},
				contract.AgentRunFinished{Run: 1, Repository: "acme/api", Outcome: contract.OutcomePR, Number: 42, At: at("00:00")},
			)
	})

	f.Run("a failed run carries no explanation", func(s *extensions.Scenario) {
		s.Covers("OPEN-2.3.2").
			Given(contract.RepositoryConnected{FullName: "acme/api"}).
			GivenAgentExplains("## Finding\n\nStray text.").
			GivenAgentFails().
			WhenWokenUpFor(time.Minute).
			Then("the end of the run is a failure with no explanation").
			ExpectFacts(
				contract.AdmissionApproved{Repository: "acme/api"},
				contract.AgentRunStarted{Run: 1, Repository: "acme/api", At: at("00:00")},
				contract.AgentRunFinished{Run: 1, Repository: "acme/api", Outcome: contract.OutcomeFailed, At: at("00:00")},
			)
	})

	for _, tt := range []struct {
		name    string
		given   func(*extensions.Scenario) *extensions.Scenario
		outcome contract.Outcome
		number  int
	}{
		{"a run with a PR", func(s *extensions.Scenario) *extensions.Scenario {
			return s.GivenAgentChanges("gc/run-1", "Remove dead code", "The helper has no callers.").GivenGitHubCreatesPR(42)
		}, contract.OutcomePR, 42},
		{"a run without a PR", func(s *extensions.Scenario) *extensions.Scenario { return s }, contract.OutcomeNoPR, 0},
		{"a failed run", func(s *extensions.Scenario) *extensions.Scenario { return s.GivenAgentFails() }, contract.OutcomeFailed, 0},
	} {
		f.Run(tt.name+" records its tokens", func(s *extensions.Scenario) {
			tokens := contract.TokenUsage{Input: 1200, Cached: 900, Output: 57}
			facts := []contract.Fact{
				contract.AdmissionApproved{Repository: "acme/api"},
				contract.AgentRunStarted{Run: 1, Repository: "acme/api", At: at("00:00")},
			}
			if tt.number != 0 {
				facts = append(facts, contract.PrCreated{Repository: "acme/api", Number: tt.number})
			}
			facts = append(facts, contract.AgentRunFinished{Run: 1, Repository: "acme/api", Outcome: tt.outcome, Number: tt.number, Tokens: tokens, At: at("00:00")})
			tt.given(s.Covers("OPEN-2.6.2").Given(contract.RepositoryConnected{FullName: "acme/api"}).GivenAgentUses(tokens)).
				WhenWokenUpFor(time.Minute).
				Then("the end of the run carries the tokens").
				ExpectFacts(facts...)
		})
	}

	f.Run("an interrupted run records no tokens", func(s *extensions.Scenario) {
		s.Covers("OPEN-2.6.2").
			Given(
				contract.RepositoryConnected{FullName: "acme/api"},
				contract.AgentRunStarted{Run: 1, Repository: "acme/api", At: at("09:00")},
			).
			GivenAgentUses(contract.TokenUsage{Input: 1200, Cached: 900, Output: 57}).
			WhenRunning("10:30", time.Minute).
			Then("the run is interrupted with no tokens").
			ExpectFacts(contract.AgentRunFinished{Run: 1, Repository: "acme/api", Outcome: contract.OutcomeInterrupted, At: at("10:30")})
	})

	f.Run("no PR when the agent fails", func(s *extensions.Scenario) {
		s.Covers("OPEN-2.4.1").
			Given(contract.RepositoryConnected{FullName: "acme/api"}).
			GivenAgentFails().
			WhenWokenUpFor(time.Minute).
			Then("the run is recorded as failed").
			ExpectFacts(
				contract.AdmissionApproved{Repository: "acme/api"},
				contract.AgentRunStarted{Run: 1, Repository: "acme/api", At: at("00:00")},
				contract.AgentRunFinished{Run: 1, Repository: "acme/api", Outcome: contract.OutcomeFailed, At: at("00:00")},
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
				contract.AgentRunStarted{Run: 1, Repository: "acme/api", At: at("00:00")},
				contract.AgentRunFinished{Run: 1, Repository: "acme/api", Outcome: contract.OutcomeFailed, At: at("00:00")},
			)
	})

	f.Run("stop a run at the limit", func(s *extensions.Scenario) {
		s.Covers("OPEN-2.5.1", "OPEN-2.6.1").
			Given(contract.RepositoryConnected{FullName: "acme/api"}).
			GivenAgentHangs().
			WhenWokenUpFor(2*time.Hour+time.Minute).
			Then("the run is stopped and recorded as timed out").
			ExpectFacts(
				contract.AdmissionApproved{Repository: "acme/api"},
				contract.AgentRunStarted{Run: 1, Repository: "acme/api", At: at("00:00")},
				contract.AgentRunFinished{Run: 1, Repository: "acme/api", Outcome: contract.OutcomeTimedOut, At: at("02:00")},
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
				contract.AgentRunStarted{Run: 1, Repository: "acme/api", At: at("00:00")},
				contract.PrCreated{Repository: "acme/api", Number: 42},
				contract.AgentRunFinished{Run: 1, Repository: "acme/api", Outcome: contract.OutcomePR, Number: 42, At: at("01:59")},
			)
	})

	f.Run("a PR that GitHub takes 59 seconds to create", func(s *extensions.Scenario) {
		s.Covers("OPEN-2.2.3").
			Given(contract.RepositoryConnected{FullName: "acme/api"}).
			GivenAgentChanges("gc/run-1", "Remove dead code", "The helper has no callers.").
			GivenGitHubCreatesPRAfter(42, 59*time.Second).
			WhenWokenUpFor(3*time.Minute).
			Then("the PR is recorded").
			ExpectFacts(
				contract.AdmissionApproved{Repository: "acme/api"},
				contract.AgentRunStarted{Run: 1, Repository: "acme/api", At: at("00:00")},
				contract.PrCreated{Repository: "acme/api", Number: 42},
				contract.AgentRunFinished{Run: 1, Repository: "acme/api", Outcome: contract.OutcomePR, Number: 42, At: at("00:00").Add(59 * time.Second)},
			)
	})

	f.Run("a PR that GitHub never answers about", func(s *extensions.Scenario) {
		s.Covers("OPEN-2.2.3", "OPEN-2.4.2").
			Given(contract.RepositoryConnected{FullName: "acme/api"}).
			GivenAgentChanges("gc/run-1", "Remove dead code", "The helper has no callers.").
			GivenGitHubHangs().
			WhenWokenUpFor(3*time.Minute).
			Then("gc gives up after 60 seconds and records a failure").
			ExpectFacts(
				contract.AdmissionApproved{Repository: "acme/api"},
				contract.AgentRunStarted{Run: 1, Repository: "acme/api", At: at("00:00")},
				contract.AgentRunFinished{Run: 1, Repository: "acme/api", Outcome: contract.OutcomeFailed, At: at("00:01")},
			).
			Expect(s.GitHubWaited(time.Minute))
	})

	f.RunWakeUp("the agent's reason reaches the log", func(s *extensions.WakeUpRun) {
		s.Covers("OPEN-2.4.3").
			GivenAgentFailsWith("the compiler is missing").
			WhenWokenUp().
			Then("the log says which repository failed and why").
			ExpectOutcome(contract.OutcomeFailed).
			ExpectLog("acme/api", "the compiler is missing")
	})

	f.RunWakeUp("GitHub's refusal reaches the log", func(s *extensions.WakeUpRun) {
		s.Covers("OPEN-2.4.4").
			GivenAgentChanged().
			GivenGitHubAnswers(http.StatusUnprocessableEntity).
			WhenWokenUp().
			Then("the log says which repository failed and what GitHub answered").
			ExpectOutcome(contract.OutcomeFailed).
			ExpectLog("acme/api", "422")
	})

	for _, tt := range []struct {
		name, form string
	}{
		{"a malformed answer", extensions.QuotedInStatus},
		{"a broken redirect", extensions.QuotedInRedirect},
	} {
		f.RunWakeUp("the token stays out of the log when creating the PR: "+tt.name, func(s *extensions.WakeUpRun) {
			s.Covers("OPEN-2.7.1").
				GivenAgentChanged().
				GivenGitHubQuotesToken(tt.form).
				WhenWokenUp().
				Then("the run fails and the log has no token").
				ExpectOutcome(contract.OutcomeFailed).
				ExpectLogWithout(s.GitHubToken())
		})

		f.RunWakeUp("the token stays out of the log when asking for the clone URL: "+tt.name, func(s *extensions.WakeUpRun) {
			s.Covers("OPEN-2.7.1").
				GivenTheRealAgentAsksForTheCloneURL().
				GivenGitHubQuotesToken(tt.form).
				WhenWokenUp().
				Then("the run fails and the log has no token").
				ExpectOutcome(contract.OutcomeFailed).
				ExpectLogWithout(s.GitHubToken())
		})
	}

	f.Run("run a connected repository now", func(s *extensions.Scenario) {
		s.Given(contract.RepositoryConnected{FullName: "acme/api"}).
			Covers("OPEN-3.1.1", "OPEN-3.1.2").
			When(spec.Post("/repositories/acme/api/run")).
			Then("the run is requested and the block says so").
			ExpectStatus(http.StatusOK).
			ExpectFacts(contract.RunRequested{Repository: "acme/api"}).
			ExpectElement(extensions.UpdatedBlock).
			ExpectElement(`#repositories-panel li[data-repository="acme/api"] button[hx-post="/repositories/acme/api/run"]`).
			ExpectText(`#repositories-panel [role="status"]`, "Run requested. Follow it on the runs page.").
			ExpectElement(`#repositories-panel [role="status"] a[href="/runs"]`)
	})

	f.Run("run a repository with no room now", func(s *extensions.Scenario) {
		s.Given(contract.RepositoryConnected{FullName: "acme/api"}).
			GivenOpenPRs("acme/api", 3).
			Given(contract.RepositorySettingsChanged{Repository: "acme/api", Pace: 1, Limit: 3, At: at("00:00")}).
			Covers("OPEN-3.1.6").
			When(spec.Post("/repositories/acme/api/run")).
			Then("the run is requested, and the block says there is no room").
			ExpectStatus(http.StatusOK).
			ExpectFacts(contract.RunRequested{Repository: "acme/api"}).
			ExpectElement(extensions.UpdatedBlock).
			ExpectText(`#repositories-panel [role="status"]`, "No room: 3 of 3 PRs are open. Merge or close some, or raise the limit in the settings.").
			ExpectElement(`#repositories-panel [role="status"] a[href="/repositories/acme/api/settings"]`).
			ExpectNoElement(`#repositories-panel [role="status"] a[href="/runs"]`)
	})

	f.Run("run a repository with one place left now", func(s *extensions.Scenario) {
		s.Given(contract.RepositoryConnected{FullName: "acme/api"}).
			GivenOpenPRs("acme/api", 2).
			Given(contract.RepositorySettingsChanged{Repository: "acme/api", Pace: 1, Limit: 3, At: at("00:00")}).
			Covers("OPEN-3.1.6").
			When(spec.Post("/repositories/acme/api/run")).
			Then("the run is requested as usual").
			ExpectStatus(http.StatusOK).
			ExpectFacts(contract.RunRequested{Repository: "acme/api"}).
			ExpectText(`#repositories-panel [role="status"]`, "Run requested. Follow it on the runs page.")
	})

	f.Run("run a repository with a working run now", func(s *extensions.Scenario) {
		s.Given(contract.RepositoryConnected{FullName: "acme/api"}).
			GivenOpenPRs("acme/api", 12).
			Given(contract.AgentRunStarted{Run: 4, Repository: "acme/api", At: at("00:00")}).
			Covers("OPEN-3.1.7").
			When(spec.Post("/repositories/acme/api/run")).
			Then("the run is requested, and the block says a run is already working").
			ExpectStatus(http.StatusOK).
			ExpectFacts(contract.RunRequested{Repository: "acme/api"}).
			ExpectElement(extensions.UpdatedBlock).
			ExpectText(`#repositories-panel [role="status"]`, "Already running: run 4. Follow it on the runs page.").
			ExpectElement(`#repositories-panel [role="status"] a[href="/runs"]`)
	})

	f.Run("run a repository with a finished run now", func(s *extensions.Scenario) {
		s.Given(contract.RepositoryConnected{FullName: "acme/api"}).
			Given(
				contract.AgentRunStarted{Run: 4, Repository: "acme/api", At: at("00:00")},
				contract.AgentRunFinished{Run: 4, Repository: "acme/api", Outcome: contract.OutcomeNoPR, At: at("00:30")},
			).
			Covers("OPEN-3.1.7").
			When(spec.Post("/repositories/acme/api/run")).
			Then("the run is requested as usual").
			ExpectStatus(http.StatusOK).
			ExpectFacts(contract.RunRequested{Repository: "acme/api"}).
			ExpectText(`#repositories-panel [role="status"]`, "Run requested. Follow it on the runs page.")
	})

	f.Run("run a repository with a pending request now", func(s *extensions.Scenario) {
		s.Given(contract.RepositoryConnected{FullName: "acme/api"}).
			GivenOpenPRs("acme/api", 12).
			Given(contract.RunRequested{Repository: "acme/api"}).
			Covers("OPEN-3.1.8").
			When(spec.Post("/repositories/acme/api/run")).
			Then("the run is requested, and the block says one is already requested").
			ExpectStatus(http.StatusOK).
			ExpectFacts(contract.RunRequested{Repository: "acme/api"}).
			ExpectElement(extensions.UpdatedBlock).
			ExpectText(`#repositories-panel [role="status"]`, "A run is already requested. Follow it on the runs page.").
			ExpectElement(`#repositories-panel [role="status"] a[href="/runs"]`)
	})

	for _, decision := range []contract.Fact{
		contract.AdmissionApproved{Repository: "acme/api"},
		contract.AdmissionDeclined{Repository: "acme/api", Open: 12, Limit: 12},
	} {
		f.Run(fmt.Sprintf("run a repository with a decided request now (%T)", decision), func(s *extensions.Scenario) {
			s.Given(contract.RepositoryConnected{FullName: "acme/api"}).
				Given(contract.RunRequested{Repository: "acme/api"}, decision).
				Covers("OPEN-3.1.8").
				When(spec.Post("/repositories/acme/api/run")).
				Then("the run is requested as usual").
				ExpectStatus(http.StatusOK).
				ExpectFacts(contract.RunRequested{Repository: "acme/api"}).
				ExpectText(`#repositories-panel [role="status"]`, "Run requested. Follow it on the runs page.")
		})
	}

	f.Run("run a repository whose other repository has a pending request now", func(s *extensions.Scenario) {
		s.Given(contract.RepositoryConnected{FullName: "acme/api"}, contract.RepositoryConnected{FullName: "acme/web"}).
			Given(contract.RunRequested{Repository: "acme/web"}, contract.AgentRunStarted{Run: 4, Repository: "acme/web", At: at("00:00")}).
			Covers("OPEN-3.1.7", "OPEN-3.1.8").
			When(spec.Post("/repositories/acme/api/run")).
			Then("the run is requested as usual").
			ExpectStatus(http.StatusOK).
			ExpectFacts(contract.RunRequested{Repository: "acme/api"}).
			ExpectText(`#repositories-panel [role="status"]`, "Run requested. Follow it on the runs page.")
	})

	f.Run("run a repository that is not connected now", func(s *extensions.Scenario) {
		s.Given(contract.RepositoryConnected{FullName: "acme/api"}, contract.RepositoryDisconnected{FullName: "acme/api"}).
			Covers("OPEN-3.1.3").
			When(spec.Post("/repositories/acme/api/run")).
			Then("the request is rejected and nothing is requested").
			ExpectStatus(http.StatusUnprocessableEntity).
			ExpectFacts().
			ExpectElement(extensions.UpdatedBlock).
			ExpectText(`#repositories-panel [role="alert"]`, "This repository is not connected")
	})

	f.Run("run a repository named in another casing now", func(s *extensions.Scenario) {
		s.Given(contract.RepositoryConnected{FullName: "acme/api"}).
			Covers("OPEN-3.1.4").
			When(spec.Post("/repositories/Acme/API/run")).
			Then("the run of acme/api is requested").
			ExpectStatus(http.StatusOK).
			ExpectFacts(contract.RunRequested{Repository: "acme/api"})
	})

	f.Run("refuse a run request from another site", func(s *extensions.Scenario) {
		s.Given(contract.RepositoryConnected{FullName: "acme/api"}).
			Covers("OPEN-3.1.5").
			When(spec.Post("/repositories/acme/api/run").WithHeader("Sec-Fetch-Site", "cross-site")).
			Then("the request is rejected and nothing is requested").
			ExpectStatus(http.StatusForbidden).
			ExpectFacts()
	})

	f.RunLaunch("a run request starts a run at once", func(s *extensions.Launch) {
		s.GivenGitHubSees("acme/api").
			GivenJournal(contract.RepositoryConnected{FullName: "acme/api"}).
			Covers("OPEN-3.2.1").
			When(spec.Post("/repositories/acme/api/run")).
			ThenTheRunEnds().
			ExpectStatus(http.StatusOK).
			ExpectJournalKinds("RepositoryConnected", "RunRequested", "AdmissionApproved", "AgentRunStarted", "AgentRunFinished")
	})

	f.RunLaunch("a run request with no room is declined at once", func(s *extensions.Launch) {
		journal := []contract.Fact{contract.RepositoryConnected{FullName: "acme/api"}}
		for number := 1; number <= 12; number++ {
			journal = append(journal, contract.PrCreated{Repository: "acme/api", Number: number})
		}
		s.GivenJournal(journal...).
			Covers("OPEN-3.2.1").
			When(spec.Post("/repositories/acme/api/run")).
			ThenAWhilePasses().
			ExpectStatus(http.StatusOK).
			ExpectJournal(append(journal, contract.RunRequested{Repository: "acme/api"}, contract.AdmissionDeclined{Repository: "acme/api", Open: 12, Limit: 12})...)
	})

	f.RunLaunch("a run request from before the start makes no decision", func(s *extensions.Launch) {
		s.GivenGitHubSees("acme/web").
			GivenJournal(contract.RepositoryConnected{FullName: "acme/api"}, contract.RunRequested{Repository: "acme/api"}).
			Covers("OPEN-3.2.2").
			When(spec.Post("/repositories").WithForm("repo", "acme/web")).
			ThenAWhilePasses().
			ExpectStatus(http.StatusOK).
			ExpectJournal(
				contract.RepositoryConnected{FullName: "acme/api"},
				contract.RunRequested{Repository: "acme/api"},
				contract.RepositoryConnected{FullName: "acme/web"},
			)
	})
}
