package specs

import (
	"net/http"
	"testing"
	"time"

	"github.com/Andrew-Sem/gc/framework/spec"
	"github.com/Andrew-Sem/gc/specs/contract"
	"github.com/Andrew-Sem/gc/specs/extensions"
)

func TestMergePullRequests(t *testing.T) {
	at := func(clock string) time.Time { return extensions.At("2000-01-01 " + clock) }
	f := extensions.NewFeature(t, "merge-pull-requests.md")
	connected := contract.RepositoryConnected{FullName: "acme/api"}
	created := contract.PrCreated{Repository: "acme/api", Number: 7}
	mergeOn := contract.RepositorySettingsChanged{Repository: "acme/api", Pace: 1, Limit: 12, Merge: true, At: extensions.At("1999-12-31 09:00")}
	open := extensions.OpenPullOnGitHub()
	merged := open
	merged.Closed, merged.UpdatedAt = true, extensions.PullMergedAt
	with := func(change func(*extensions.PullOnGitHub)) extensions.PullOnGitHub {
		pull := open
		change(&pull)
		return pull
	}

	f.Run("the settings form has merging off by default", func(s *extensions.Scenario) {
		s.Covers("MERGE-1.1.1").
			Given(connected).
			When(spec.Get("/repositories/acme/api/settings")).
			Then("the checkbox is there and not checked").
			ExpectStatus(http.StatusOK).
			ExpectFacts().
			ExpectText(`#settings-panel label:has(input[type="checkbox"][name="merge"])`, "Merge green PRs").
			ExpectNoElement(`#settings-panel input[name="merge"][checked]`)
	})

	f.Run("the settings form shows merging on", func(s *extensions.Scenario) {
		s.Covers("MERGE-1.1.1").
			Given(connected, mergeOn).
			When(spec.Get("/repositories/acme/api/settings")).
			Then("the checkbox is checked").
			ExpectStatus(http.StatusOK).
			ExpectFacts().
			ExpectElement(`#settings-panel input[type="checkbox"][name="merge"][checked]`)
	})

	f.Run("turn merging on", func(s *extensions.Scenario) {
		s.Covers("MERGE-1.1.1", "MERGE-1.1.2").
			Given(connected).
			When(spec.Post("/repositories/acme/api/settings").WithForm("pace", "1").WithForm("limit", "12").WithForm("merge", "on")).
			Then("merging is saved, and the form shows it on").
			ExpectStatus(http.StatusOK).
			ExpectFacts(contract.RepositorySettingsChanged{Repository: "acme/api", Pace: 1, Limit: 12, Merge: true, At: at("00:00")}).
			ExpectElement(`#settings-panel input[name="merge"][checked]`)
	})

	f.Run("turn merging off", func(s *extensions.Scenario) {
		s.Covers("MERGE-1.1.2").
			Given(connected, mergeOn).
			When(spec.Post("/repositories/acme/api/settings").WithForm("pace", "1").WithForm("limit", "12")).
			Then("merging off is saved").
			ExpectStatus(http.StatusOK).
			ExpectFacts(contract.RepositorySettingsChanged{Repository: "acme/api", Pace: 1, Limit: 12, At: at("00:00")}).
			ExpectNoElement(`#settings-panel input[name="merge"][checked]`)
	})

	f.Run("save merging that is already on", func(s *extensions.Scenario) {
		s.Covers("MERGE-1.1.2").
			Given(connected, mergeOn).
			When(spec.Post("/repositories/acme/api/settings").WithForm("pace", "1").WithForm("limit", "12").WithForm("merge", "on")).
			Then("nothing is written").
			ExpectStatus(http.StatusOK).
			ExpectFacts()
	})

	f.Run("the list says which repositories merge", func(s *extensions.Scenario) {
		s.Covers("MERGE-1.1.3").
			Given(connected, contract.RepositoryConnected{FullName: "acme/web"}, mergeOn).
			When(spec.Get("/repositories")).
			Then("acme/api merges, acme/web does not").
			ExpectStatus(http.StatusOK).
			ExpectFacts().
			ExpectText(`#repositories li[data-repository="acme/api"]`, "acme/api every 1 h, at most 12 open, merges green PRs Run now Disconnect Settings").
			ExpectText(`#repositories li[data-repository="acme/web"]`, "acme/web every 1 h, at most 12 open Run now Disconnect Settings")
	})

	f.Run("a sweep comes at 06:00 before the decision", func(s *extensions.Scenario) {
		s.Covers("MERGE-2.1.1", "MERGE-3.2.2").
			Given(connected, mergeOn, created).
			GivenPullOnGitHub("acme/api", 7, open).
			WhenRunning("05:59", 2*time.Minute).
			Then("the PR is merged, and the decision after it sees no open PR").
			ExpectFacts(
				open.Snapshot("acme/api", 7),
				contract.MergeApproved{Repository: "acme/api", Number: 7, HeadSHA: "abc123"},
				merged.Snapshot("acme/api", 7),
				contract.AdmissionApproved{Repository: "acme/api"},
				contract.AgentRunStarted{Run: 1, Repository: "acme/api", At: at("06:00")},
				contract.AgentRunFinished{Run: 1, Repository: "acme/api", Outcome: contract.OutcomeNoPR, At: at("06:00")},
			)
	})

	f.Run("no sweep between the sweep hours", func(s *extensions.Scenario) {
		s.Covers("MERGE-2.1.1").
			Given(connected, mergeOn, created).
			GivenPullOnGitHub("acme/api", 7, open).
			WhenRunning("06:59", 2*time.Minute).
			Then("07:00 only decides").
			ExpectFacts(
				contract.AdmissionApproved{Repository: "acme/api"},
				contract.AgentRunStarted{Run: 1, Repository: "acme/api", At: at("07:00")},
				contract.AgentRunFinished{Run: 1, Repository: "acme/api", Outcome: contract.OutcomeNoPR, At: at("07:00")},
			).
			Expect(s.GitHubMergedNothing())
	})

	f.Run("no sweep right after the start", func(s *extensions.Scenario) {
		s.Covers("MERGE-2.1.2").
			Given(connected, mergeOn, created).
			GivenPullOnGitHub("acme/api", 7, open).
			WhenRunning("06:20", 30*time.Minute).
			Then("the missed 06:00 sweep is not made up").
			ExpectFacts().
			Expect(s.GitHubNotAsked())
	})

	f.Run("a repository not due is still swept", func(s *extensions.Scenario) {
		s.Covers("MERGE-2.1.3").
			Given(connected, created, contract.RepositorySettingsChanged{Repository: "acme/api", Pace: 5, Limit: 12, Merge: true, At: extensions.At("2000-01-01 04:00")}).
			GivenPullOnGitHub("acme/api", 7, open).
			WhenRunning("05:59", 2*time.Minute).
			Then("the PR is merged, and no decision is made").
			ExpectFacts(
				open.Snapshot("acme/api", 7),
				contract.MergeApproved{Repository: "acme/api", Number: 7, HeadSHA: "abc123"},
				merged.Snapshot("acme/api", 7),
			)
	})

	f.Run("a disconnected repository is not swept", func(s *extensions.Scenario) {
		s.Covers("MERGE-2.1.3").
			Given(connected, mergeOn, created, contract.RepositoryDisconnected{FullName: "acme/api"}).
			GivenPullOnGitHub("acme/api", 7, open).
			WhenSwept().
			Then("nothing is read or merged").
			ExpectFacts().
			Expect(s.GitHubNotAsked())
	})

	f.Run("a sweep reads the PRs with merging off", func(s *extensions.Scenario) {
		s.Covers("MERGE-2.2.1", "MERGE-2.2.2", "MERGE-3.1.1").
			Given(connected, created).
			GivenPullOnGitHub("acme/api", 7, open).
			WhenSwept().
			Then("the state is recorded, and nothing is decided or merged").
			ExpectFacts(open.Snapshot("acme/api", 7)).
			Expect(s.GitHubMergedNothing())
	})

	f.Run("a PR closed on GitHub frees its place", func(s *extensions.Scenario) {
		closed := with(func(p *extensions.PullOnGitHub) { p.Closed = true })
		s.Covers("MERGE-2.2.2", "MERGE-2.2.4").
			Given(connected, mergeOn, created).
			GivenPullOnGitHub("acme/api", 7, closed).
			Given(contract.RepositorySettingsChanged{Repository: "acme/api", Pace: 1, Limit: 1, Merge: true, At: extensions.At("1999-12-31 09:00")}).
			WhenRunning("05:59", 2*time.Minute).
			Then("the closed state is recorded with no merge decision, and the decision after it sees no open PR").
			ExpectFacts(
				closed.Snapshot("acme/api", 7),
				contract.AdmissionApproved{Repository: "acme/api"},
				contract.AgentRunStarted{Run: 1, Repository: "acme/api", At: at("06:00")},
				contract.AgentRunFinished{Run: 1, Repository: "acme/api", Outcome: contract.OutcomeNoPR, At: at("06:00")},
			).
			Expect(s.GitHubMergedNothing())
	})

	f.Run("a known state writes nothing", func(s *extensions.Scenario) {
		s.Covers("MERGE-2.2.2").
			Given(connected, created, open.Snapshot("acme/api", 7)).
			GivenPullOnGitHub("acme/api", 7, open).
			WhenSwept().
			Then("nothing is written").
			ExpectFacts()
	})

	f.Run("a PR that GitHub does not give", func(s *extensions.Scenario) {
		s.Covers("MERGE-2.2.3").
			Given(connected, mergeOn, created).
			GivenPullOnGitHub("acme/api", 7, with(func(p *extensions.PullOnGitHub) { p.Unreadable = true })).
			WhenSwept().
			Then("the merge is declined, with no state").
			ExpectFacts(contract.MergeDeclined{Repository: "acme/api", Number: 7, Reason: "GitHub did not give the PR"}).
			Expect(s.GitHubMergedNothing())
	})

	f.Run("a PR read without a head", func(s *extensions.Scenario) {
		s.Covers("MERGE-2.2.3").
			Given(connected, mergeOn, created).
			GivenPullOnGitHub("acme/api", 7, with(func(p *extensions.PullOnGitHub) { p.Incomplete = true })).
			WhenSwept().
			Then("the merge is declined, with no state").
			ExpectFacts(contract.MergeDeclined{Repository: "acme/api", Number: 7, Reason: "GitHub did not give the PR"}).
			Expect(s.GitHubMergedNothing())
	})

	f.Run("a PR that GitHub does not give with merging off", func(s *extensions.Scenario) {
		s.Covers("MERGE-2.2.3").
			Given(connected, created).
			GivenPullOnGitHub("acme/api", 7, with(func(p *extensions.PullOnGitHub) { p.Unreadable = true })).
			WhenSwept().
			Then("nothing is written").
			ExpectFacts()
	})

	f.Run("merge a green PR", func(s *extensions.Scenario) {
		s.Covers("MERGE-3.1.1", "MERGE-3.1.4", "MERGE-3.2.1", "MERGE-3.2.2").
			Given(connected, mergeOn, created).
			GivenPullOnGitHub("acme/api", 7, open).
			WhenSwept().
			Then("the PR is approved, merged by squash at the read head, and recorded as closed").
			ExpectFacts(
				open.Snapshot("acme/api", 7),
				contract.MergeApproved{Repository: "acme/api", Number: 7, HeadSHA: "abc123"},
				merged.Snapshot("acme/api", 7),
			).
			Expect(s.GitHubMerged("acme/api", 7, "abc123"))
	})

	f.Run("only our PRs are swept", func(s *extensions.Scenario) {
		s.Covers("MERGE-2.2.1").
			Given(connected, mergeOn).
			GivenPullOnGitHub("acme/api", 7, open).
			WhenSwept().
			Then("a PR without PrCreated is not read").
			ExpectFacts().
			Expect(s.GitHubNotAsked())
	})

	for _, tt := range []struct {
		name   string
		change func(*extensions.PullOnGitHub)
		reason string
	}{
		{"a draft", func(p *extensions.PullOnGitHub) { p.Draft = true; p.MergeableState = "draft" }, "draft"},
		{"a PR GitHub is still checking", func(p *extensions.PullOnGitHub) { p.Mergeable = "null"; p.MergeableState = "unknown" }, "GitHub is still checking"},
		{"a PR with conflicts", func(p *extensions.PullOnGitHub) { p.Mergeable = "false"; p.MergeableState = "dirty" }, "conflicts"},
		{"a PR whose required checks have not passed", func(p *extensions.PullOnGitHub) { p.MergeableState = "blocked" }, "required checks have not passed"},
		{"a PR behind its base", func(p *extensions.PullOnGitHub) { p.MergeableState = "behind" }, "behind the base branch"},
		{"a PR in another state", func(p *extensions.PullOnGitHub) { p.MergeableState = "strange" }, "not mergeable: strange"},
	} {
		f.Run("do not merge "+tt.name, func(s *extensions.Scenario) {
			pull := with(tt.change)
			s.Covers("MERGE-3.1.1", "MERGE-3.1.2").
				Given(connected, mergeOn, created).
				GivenPullOnGitHub("acme/api", 7, pull).
				WhenSwept().
				Then("the merge is declined with the reason").
				ExpectFacts(
					pull.Snapshot("acme/api", 7),
					contract.MergeDeclined{Repository: "acme/api", Number: 7, Reason: tt.reason},
				).
				Expect(s.GitHubMergedNothing())
		})
	}

	for _, state := range []string{"unstable", "has_hooks"} {
		f.Run("merge a PR in the state "+state, func(s *extensions.Scenario) {
			pull := with(func(p *extensions.PullOnGitHub) { p.MergeableState = state })
			s.Covers("MERGE-3.1.2", "MERGE-3.1.4").
				Given(connected, mergeOn, created).
				GivenPullOnGitHub("acme/api", 7, pull).
				WhenSwept().
				Then("the PR is approved and merged").
				ExpectFacts(
					pull.Snapshot("acme/api", 7),
					contract.MergeApproved{Repository: "acme/api", Number: 7, HeadSHA: "abc123"},
					merged.Snapshot("acme/api", 7),
				).
				Expect(s.GitHubMerged("acme/api", 7, "abc123"))
		})
	}

	f.Run("the first failing condition gives the reason", func(s *extensions.Scenario) {
		pull := with(func(p *extensions.PullOnGitHub) { p.Draft = true; p.Mergeable = "false"; p.MergeableState = "dirty" })
		s.Covers("MERGE-3.1.2").
			Given(connected, mergeOn, created).
			GivenPullOnGitHub("acme/api", 7, pull).
			GivenRequiredChecks().
			WhenSwept().
			Then("a draft with conflicts and no required checks is declined as a draft").
			ExpectFacts(
				pull.Snapshot("acme/api", 7),
				contract.MergeDeclined{Repository: "acme/api", Number: 7, Reason: "draft"},
			)
	})

	f.Run("do not merge without required checks", func(s *extensions.Scenario) {
		s.Covers("MERGE-3.1.2").
			Given(connected, mergeOn, created).
			GivenPullOnGitHub("acme/api", 7, open).
			GivenRequiredChecks().
			WhenSwept().
			Then("the merge is declined, even though GitHub says clean").
			ExpectFacts(
				open.Snapshot("acme/api", 7),
				contract.MergeDeclined{Repository: "acme/api", Number: 7, Reason: "no required checks"},
			).
			Expect(s.GitHubMergedNothing())
	})

	f.Run("a required check given by app counts", func(s *extensions.Scenario) {
		s.Covers("MERGE-3.1.2").
			Given(connected, mergeOn, created).
			GivenPullOnGitHub("acme/api", 7, open).
			GivenRequiredCheckApps("ci/build").
			WhenSwept().
			Then("the PR is merged").
			ExpectFacts(
				open.Snapshot("acme/api", 7),
				contract.MergeApproved{Repository: "acme/api", Number: 7, HeadSHA: "abc123"},
				merged.Snapshot("acme/api", 7),
			)
	})

	f.Run("a base branch that GitHub does not give", func(s *extensions.Scenario) {
		s.Covers("MERGE-3.1.3").
			Given(connected, mergeOn, created).
			GivenPullOnGitHub("acme/api", 7, open).
			GivenBaseBranchUnreadable().
			WhenSwept().
			Then("the merge is declined").
			ExpectFacts(
				open.Snapshot("acme/api", 7),
				contract.MergeDeclined{Repository: "acme/api", Number: 7, Reason: "GitHub did not give the base branch"},
			).
			Expect(s.GitHubMergedNothing())
	})

	f.Run("GitHub refuses the merge", func(s *extensions.Scenario) {
		s.Covers("MERGE-3.2.3").
			Given(connected, mergeOn, created).
			GivenPullOnGitHub("acme/api", 7, open).
			GivenGitHubRefusesMerge(http.StatusConflict).
			WhenSwept().
			Then("the merge failed with GitHub's status, and the PR stays open").
			ExpectFacts(
				open.Snapshot("acme/api", 7),
				contract.MergeApproved{Repository: "acme/api", Number: 7, HeadSHA: "abc123"},
				contract.MergeFailed{Repository: "acme/api", Number: 7, Reason: "GitHub answered 409"},
			)
	})

	f.Run("GitHub does not answer the merge", func(s *extensions.Scenario) {
		s.Covers("MERGE-3.2.3").
			Given(connected, mergeOn, created).
			GivenPullOnGitHub("acme/api", 7, open).
			GivenMergeHangs().
			WhenSwept().
			Then("gc gives up after 60 seconds").
			ExpectFacts(
				open.Snapshot("acme/api", 7),
				contract.MergeApproved{Repository: "acme/api", Number: 7, HeadSHA: "abc123"},
				contract.MergeFailed{Repository: "acme/api", Number: 7, Reason: "GitHub did not answer"},
			).
			Expect(s.GitHubWaited(time.Minute))
	})

	f.Run("no GitHub answer in the journal after a sweep", func(s *extensions.Scenario) {
		s.Covers("MERGE-3.2.4").
			Given(connected, mergeOn, created).
			GivenPullOnGitHub("acme/api", 7, open).
			GivenGitHubRefusesMerge(http.StatusMethodNotAllowed).
			GivenGitHubEchoesToken().
			WhenSwept().
			Then("the failure has the status and no token").
			ExpectFacts(
				open.Snapshot("acme/api", 7),
				contract.MergeApproved{Repository: "acme/api", Number: 7, HeadSHA: "abc123"},
				contract.MergeFailed{Repository: "acme/api", Number: 7, Reason: "GitHub answered 405"},
			)
	})
}
