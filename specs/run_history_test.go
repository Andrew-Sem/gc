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

	f.Run("the page does not refresh itself", func(s *extensions.Scenario) {
		s.Covers("HISTORY-1.1.6").
			Given(startedFact).
			When(spec.Get("/runs")).
			Then("nothing on the page asks for an update").
			ExpectStatus(http.StatusOK).
			ExpectFacts().
			ExpectNoElement(`[hx-get], [hx-trigger], meta[http-equiv="refresh"]`)
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
}
