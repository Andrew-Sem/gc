package specs

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/Andrew-Sem/gc/framework/spec"
	"github.com/Andrew-Sem/gc/specs/contract"
	"github.com/Andrew-Sem/gc/specs/extensions"
	"github.com/Andrew-Sem/gc/ui"
)

func runCell(run int, name string) string {
	return `#runs tr[data-run="` + strconv.Itoa(run) + `"] td[data-cell="` + name + `"]`
}

func TestRunHistory(t *testing.T) {
	f := extensions.NewFeature(t, "run-history.md")
	started := extensions.At("1999-12-31 22:00")
	startedFact := contract.AgentRunStarted{Run: 1, Repository: "acme/api", At: started}

	f.Run("no runs yet", func(s *extensions.Scenario) {
		s.Covers("HISTORY-1.1.3", "HISTORY-1.1.5").
			Given(contract.RepositoryConnected{FullName: "acme/api"}).
			When(spec.Get("/runs")).
			Then("the page says so and has no rows").
			ExpectStatus(http.StatusOK).
			ExpectFacts().
			ExpectBodyContaining("No runs yet").
			ExpectElementCount(`#runs tr[data-run]`, 0)
	})

	f.Run("a run is on the page", func(s *extensions.Scenario) {
		s.Covers("HISTORY-1.1.1", "HISTORY-1.1.5", "HISTORY-1.2.1", "HISTORY-1.2.2").
			Given(startedFact).
			When(spec.Get("/runs")).
			Then("its row shows the repository and the start in UTC").
			ExpectStatus(http.StatusOK).
			ExpectFacts().
			ExpectElementCount(`#runs tr[data-run]`, 1).
			ExpectText(runCell(1, "repository"), "acme/api").
			ExpectText(runCell(1, "started"), "1999-12-31 22:00 UTC")
	})

	f.Run("the newest run comes first", func(s *extensions.Scenario) {
		s.Covers("HISTORY-1.1.2").
			Given(
				contract.AgentRunStarted{Run: 1, Repository: "acme/api", At: started},
				contract.AgentRunStarted{Run: 2, Repository: "acme/web", At: started},
				contract.AgentRunStarted{Run: 3, Repository: "acme/api", At: started},
			).
			When(spec.Get("/runs")).
			Then("the rows go 3, 2, 1").
			ExpectStatus(http.StatusOK).
			ExpectFacts().
			ExpectElement(`#runs tr[data-run="3"]:first-of-type`).
			ExpectElement(`#runs tr[data-run="3"] + tr[data-run="2"] + tr[data-run="1"]`)
	})

	f.Run("the runs page uses the shared interface head", func(s *extensions.Scenario) {
		s.Covers("HISTORY-1.1.4").
			When(spec.Get("/runs")).
			Then("htmx and its response rules come from ui").
			ExpectStatus(http.StatusOK).
			ExpectFacts().
			ExpectElement(`meta[name="htmx-config"][content='` + ui.HTMXConfig + `']`).
			ExpectElement(`script[src="` + ui.HTMXPath + `"]`)
	})

	f.Run("a run of a disconnected repository stays", func(s *extensions.Scenario) {
		s.Covers("HISTORY-1.2.3").
			Given(
				startedFact,
				contract.AgentRunFinished{Run: 1, Repository: "acme/api", Outcome: contract.OutcomeNoPR, At: started},
				contract.RepositoryDisconnected{FullName: "acme/api"},
			).
			When(spec.Get("/runs")).
			Then("its row is still there").
			ExpectStatus(http.StatusOK).
			ExpectFacts().
			ExpectText(runCell(1, "repository"), "acme/api")
	})

	f.Run("two runs of one repository at the same time", func(s *extensions.Scenario) {
		s.Covers("HISTORY-1.2.4").
			Given(
				contract.AgentRunStarted{Run: 1, Repository: "acme/api", At: started},
				contract.AgentRunStarted{Run: 2, Repository: "acme/api", At: extensions.At("1999-12-31 23:00")},
				contract.AgentRunFinished{Run: 1, Repository: "acme/api", Outcome: contract.OutcomeNoPR, At: extensions.At("1999-12-31 23:30")},
			).
			When(spec.Get("/runs")).
			Then("the first has ended and the second is still running").
			ExpectStatus(http.StatusOK).
			ExpectFacts().
			ExpectElementCount(`#runs tr[data-run]`, 2).
			ExpectText(runCell(1, "status"), "no PR").
			ExpectText(runCell(2, "status"), "running")
	})

	f.Run("a running run", func(s *extensions.Scenario) {
		s.Covers("HISTORY-1.3.1", "HISTORY-1.4.2").
			Given(contract.AgentRunStarted{Run: 1, Repository: "acme/api", At: extensions.At("1999-12-31 23:48")}).
			When(spec.Get("/runs")).
			Then("it is running, for the 12 minutes until the page opened").
			ExpectStatus(http.StatusOK).
			ExpectFacts().
			ExpectText(runCell(1, "status"), "running").
			ExpectText(runCell(1, "duration"), "12 min")
	})

	f.Run("a run that opened a PR", func(s *extensions.Scenario) {
		s.Covers("HISTORY-1.3.2", "HISTORY-1.4.1").
			Given(
				startedFact,
				contract.AgentRunFinished{Run: 1, Repository: "acme/api", Outcome: contract.OutcomePR, Number: 42, At: extensions.At("1999-12-31 22:41")},
			).
			When(spec.Get("/runs")).
			Then("the status links to the PR, and the duration ends with the run").
			ExpectStatus(http.StatusOK).
			ExpectFacts().
			ExpectText(runCell(1, "status")+` a[href="https://github.com/acme/api/pull/42"]`, "PR #42").
			ExpectText(runCell(1, "duration"), "41 min")
	})

	for _, tt := range []struct {
		name    string
		clause  string
		outcome contract.Outcome
		status  string
	}{
		{"a run that opened no PR", "HISTORY-1.3.3", contract.OutcomeNoPR, "no PR"},
		{"a run that failed", "HISTORY-1.3.4", contract.OutcomeFailed, "failed"},
		{"a run that timed out", "HISTORY-1.3.5", contract.OutcomeTimedOut, "timed out"},
		{"a run that was interrupted", "HISTORY-1.3.7", contract.OutcomeInterrupted, "interrupted"},
	} {
		f.Run(tt.name, func(s *extensions.Scenario) {
			s.Covers(tt.clause).
				Given(
					startedFact,
					contract.AgentRunFinished{Run: 1, Repository: "acme/api", Outcome: tt.outcome, At: extensions.At("1999-12-31 22:41")},
				).
				When(spec.Get("/runs")).
				Then("the status says so, with no link").
				ExpectStatus(http.StatusOK).
				ExpectFacts().
				ExpectText(runCell(1, "status"), tt.status).
				ExpectNoElement(runCell(1, "status") + " a")
		})
	}

	f.Run("a run without a PR says why", func(s *extensions.Scenario) {
		s.Covers("HISTORY-1.5.1").
			Given(
				startedFact,
				contract.AgentRunFinished{Run: 1, Repository: "acme/api", Outcome: contract.OutcomeNoPR, Explanation: "## Finding\n\nNothing safe to fix.", At: started},
			).
			When(spec.Get("/runs")).
			Then("the status says no PR, and the explanation opens under it").
			ExpectStatus(http.StatusOK).
			ExpectFacts().
			ExpectText(runCell(1, "status"), "no PR why## Finding Nothing safe to fix.").
			ExpectText(runCell(1, "status")+" details[data-explanation] > summary", "why").
			ExpectText(runCell(1, "status")+" details[data-explanation] > pre", "## Finding Nothing safe to fix.").
			ExpectBodyContaining("<pre>## Finding\n\nNothing safe to fix.</pre>")
	})

	f.Run("a run without a PR and without an explanation", func(s *extensions.Scenario) {
		s.Covers("HISTORY-1.5.2").
			Given(
				startedFact,
				contract.AgentRunFinished{Run: 1, Repository: "acme/api", Outcome: contract.OutcomeNoPR, At: started},
			).
			When(spec.Get("/runs")).
			Then("the status is the words alone").
			ExpectStatus(http.StatusOK).
			ExpectFacts().
			ExpectText(runCell(1, "status"), "no PR").
			ExpectNoElement(runCell(1, "status") + " details")
	})

	f.Run("markup in an explanation is shown, not run", func(s *extensions.Scenario) {
		s.Covers("HISTORY-1.5.3").
			Given(
				startedFact,
				contract.AgentRunFinished{Run: 1, Repository: "acme/api", Outcome: contract.OutcomeNoPR, Explanation: `<script>alert(1)</script><b>bold</b>`, At: started},
			).
			When(spec.Get("/runs")).
			Then("the markup is text in the pre").
			ExpectStatus(http.StatusOK).
			ExpectFacts().
			ExpectText(runCell(1, "status")+" details[data-explanation] > pre", "<script>alert(1)</script><b>bold</b>").
			ExpectNoElement(runCell(1, "status") + " details script").
			ExpectNoElement(runCell(1, "status") + " details b")
	})

	f.Run("an interrupted run has no known duration", func(s *extensions.Scenario) {
		s.Covers("HISTORY-1.4.4").
			Given(
				startedFact,
				contract.AgentRunFinished{Run: 1, Repository: "acme/api", Outcome: contract.OutcomeInterrupted, At: extensions.At("1999-12-31 23:00")},
			).
			When(spec.Get("/runs")).
			Then("the duration reads unknown").
			ExpectStatus(http.StatusOK).
			ExpectFacts().
			ExpectText(runCell(1, "duration"), "unknown")
	})

	f.Run("a failed run shows no reason", func(s *extensions.Scenario) {
		s.Covers("HISTORY-1.3.6").
			Given(
				startedFact,
				contract.AgentRunFinished{Run: 1, Repository: "acme/api", Outcome: contract.OutcomeFailed, At: started},
			).
			When(spec.Get("/runs")).
			Then("the row has only the four cells, and the status only the word").
			ExpectStatus(http.StatusOK).
			ExpectFacts().
			ExpectElementCount(`#runs tr[data-run="1"] td`, 4).
			ExpectText(runCell(1, "status"), "failed")
	})

	for _, tt := range []struct {
		name     string
		end      string
		duration string
	}{
		{"a duration under a minute", "1999-12-31 22:00", "under a minute"},
		{"a duration of one minute", "1999-12-31 22:01", "1 min"},
		{"a duration under an hour", "1999-12-31 22:59", "59 min"},
		{"a duration on a whole hour", "1999-12-31 23:00", "1 h"},
		{"a duration with hours and minutes", "1999-12-31 23:05", "1 h 5 min"},
	} {
		f.Run(tt.name, func(s *extensions.Scenario) {
			s.Covers("HISTORY-1.4.3").
				Given(
					startedFact,
					contract.AgentRunFinished{Run: 1, Repository: "acme/api", Outcome: contract.OutcomeNoPR, At: extensions.At(tt.end)},
				).
				When(spec.Get("/runs")).
				Then("it reads "+tt.duration).
				ExpectStatus(http.StatusOK).
				ExpectFacts().
				ExpectText(runCell(1, "duration"), tt.duration)
		})
	}

	f.Run("the repositories page links to the runs", func(s *extensions.Scenario) {
		s.Covers("HISTORY-2.1.1").
			When(spec.Get("/repositories")).
			Then("the link is there").
			ExpectStatus(http.StatusOK).
			ExpectFacts().
			ExpectElement(`a[href="/runs"]`)
	})

	f.Run("the runs page links to the repositories", func(s *extensions.Scenario) {
		s.Covers("HISTORY-2.1.2").
			When(spec.Get("/runs")).
			Then("the link is there").
			ExpectStatus(http.StatusOK).
			ExpectFacts().
			ExpectElement(`a[href="/repositories"]`)
	})

	const livePanel = `#runs-panel[hx-get="/runs/panel"][hx-trigger="runs-changed from:body, every 60s"][hx-swap="outerHTML"]`
	connected := contract.RepositoryConnected{FullName: "acme/api"}
	disconnected := contract.RepositoryDisconnected{FullName: "acme/api"}

	f.Run("the panel alone shows the runs", func(s *extensions.Scenario) {
		s.Covers("HISTORY-3.1.1", "HISTORY-3.1.2", "HISTORY-3.1.4").
			Given(startedFact).
			When(spec.Get("/runs/panel")).
			Then("it has the same row and keeps updating, with no page around it").
			ExpectStatus(http.StatusOK).
			ExpectFacts().
			ExpectElement(livePanel).
			ExpectText(runCell(1, "repository"), "acme/api").
			ExpectNoElement(extensions.WholePage).
			ExpectBodyWithout("EventSource")
	})

	f.Run("the panel alone with no runs", func(s *extensions.Scenario) {
		s.Covers("HISTORY-3.1.1", "HISTORY-3.1.2").
			When(spec.Get("/runs/panel")).
			Then("it says so and has no rows").
			ExpectStatus(http.StatusOK).
			ExpectFacts().
			ExpectElement(livePanel).
			ExpectBodyContaining("No runs yet").
			ExpectElementCount(`#runs tr[data-run]`, 0)
	})

	f.Run("the page holds the live panel", func(s *extensions.Scenario) {
		s.Covers("HISTORY-3.1.2", "HISTORY-3.1.3").
			Given(startedFact).
			When(spec.Get("/runs")).
			Then("the panel updates on a signal and once a minute").
			ExpectStatus(http.StatusOK).
			ExpectFacts().
			ExpectElement(livePanel + ` #runs tr[data-run="1"]`).
			ExpectElement(`#runs-panel[hx-trigger$="every 60s"]`)
	})

	f.Run("the page listens to the stream", func(s *extensions.Scenario) {
		s.Covers("HISTORY-3.3.1", "HISTORY-3.3.2", "HISTORY-3.3.3").
			When(spec.Get("/runs")).
			Then("a signal and each connection fire runs-changed on the body").
			ExpectStatus(http.StatusOK).
			ExpectFacts().
			ExpectBodyContaining(`new EventSource("/runs/events")`).
			ExpectBodyContaining(`document.body.dispatchEvent(new Event("runs-changed"))`).
			ExpectBodyContaining(`.onmessage = runsChanged`).
			ExpectBodyContaining(`.onopen = runsChanged`)
	})

	f.Run("the repositories page does not listen to the stream", func(s *extensions.Scenario) {
		s.Covers("HISTORY-3.3.4").
			When(spec.Get("/repositories")).
			Then("it opens no stream").
			ExpectStatus(http.StatusOK).
			ExpectFacts().
			ExpectBodyWithout("EventSource")
	})

	f.Run("an open stream with no new facts", func(s *extensions.Scenario) {
		s.Covers("HISTORY-3.2.1", "HISTORY-3.2.4", "HISTORY-3.2.7", "HISTORY-3.2.8").
			Given(connected).
			WhenEventStreamsOpen(1).
			Then("the headers are out at once, nothing is sent, and it ends when the client leaves").
			ExpectFacts().
			Expect(s.StreamOpenedAtOnce()).
			Expect(s.StreamsSent(1, extensions.Signals(0))).
			Expect(s.StreamsEnded())
	})

	f.Run("each new fact sends a signal", func(s *extensions.Scenario) {
		s.Covers("HISTORY-3.2.2", "HISTORY-3.2.5").
			WhenEventStreamsOpen(1, []contract.Fact{connected}, []contract.Fact{startedFact}).
			Then("two signals are sent and nothing else").
			ExpectFacts(connected, startedFact).
			Expect(s.StreamsSent(1, extensions.Signals(2)))
	})

	f.Run("a fact unrelated to runs sends a signal too", func(s *extensions.Scenario) {
		s.Covers("HISTORY-3.2.2").
			Given(connected).
			WhenEventStreamsOpen(1, []contract.Fact{disconnected}).
			Then("one signal is sent").
			ExpectFacts(disconnected).
			Expect(s.StreamsSent(1, extensions.Signals(1)))
	})

	f.Run("a fact written while the stream is busy", func(s *extensions.Scenario) {
		s.Covers("HISTORY-3.2.3").
			WhenEventStreamBusyWhileWritten(connected, disconnected).
			Then("another signal follows the last fact").
			ExpectFacts(connected, disconnected).
			Expect(s.StreamsSent(1, extensions.Signals(2)))
	})

	f.Run("every open stream gets the signal", func(s *extensions.Scenario) {
		s.Covers("HISTORY-3.2.6", "HISTORY-3.2.8").
			WhenEventStreamsOpen(2, []contract.Fact{connected}).
			Then("both streams send it, and both end when their clients leave").
			ExpectFacts(connected).
			Expect(s.StreamsSent(2, extensions.Signals(1))).
			Expect(s.StreamsEnded())
	})

	f.RunLaunch("a signal reaches the client over the network", func(s *extensions.Launch) {
		s.GivenGitHubSees("acme/api").
			Covers("HISTORY-3.2.2").
			WhenEventStreamOpenDuring(spec.Post("/repositories").WithForm("repo", "acme/api")).
			Then("connecting a repository sends the signal right away").
			ExpectServing().
			ExpectStreamSent(extensions.Signals(1))
	})

	f.RunLaunch("an open stream does not hold gc back", func(s *extensions.Launch) {
		s.Covers("HISTORY-3.2.9").
			WhenEventStreamOpen().
			Then("gc stops with the client still connected").
			ExpectServing()
	})
}
