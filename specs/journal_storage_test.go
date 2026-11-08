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

func TestJournalStorage(t *testing.T) {
	f := extensions.NewFeature(t, "journal-storage.md")
	connect := func(repo string) spec.Request { return spec.Post("/repositories").WithForm("repo", repo) }
	repository := func(repo string) string { return `#repositories-panel li[data-repository="` + repo + `"]` }
	connected := contract.RepositoryConnected{FullName: "acme/api"}
	started := extensions.At("1999-12-31 22:00")
	running := contract.AgentRunStarted{Run: 1, Repository: "acme/api", At: started}

	f.RunLaunch("every kind of fact survives a restart", func(s *extensions.Launch) {
		s.GivenJournal(extensions.EveryKindOfFact()...).
			Covers("STORE-1.1.1").
			WhenStarted().
			Then("the journal is back in the same order with equal fields").
			ExpectServing().
			ExpectJournal(extensions.EveryKindOfFact()...)
	})

	f.RunLaunch("connected repositories survive a restart", func(s *extensions.Launch) {
		s.GivenGitHubSees("acme/api", "acme/web").
			GivenEarlierStart(connect("acme/api"), connect("acme/web")).
			Covers("STORE-1.1.2").
			When(spec.Get("/repositories")).
			Then("both are listed, the newest first").
			ExpectStatus(http.StatusOK).
			ExpectElement(repository("acme/web") + ` + li[data-repository="acme/api"]`)
	})

	f.RunLaunch("a disconnected repository stays disconnected after a restart", func(s *extensions.Launch) {
		s.GivenGitHubSees("acme/api").
			GivenEarlierStart(connect("acme/api"), spec.Post("/repositories/acme/api/disconnect")).
			Covers("STORE-1.1.3").
			When(spec.Get("/repositories")).
			Then("the repository is not listed").
			ExpectStatus(http.StatusOK).
			ExpectNoElement(repository("acme/api"))
	})

	f.RunLaunch("runs survive a restart", func(s *extensions.Launch) {
		s.GivenJournal(running, contract.AgentRunFinished{Run: 1, Repository: "acme/api", Outcome: contract.OutcomeNoPR, At: started}).
			Covers("STORE-1.1.4").
			When(spec.Get("/runs")).
			Then("the run is on the page").
			ExpectStatus(http.StatusOK).
			ExpectElement(`#runs tr[data-run="1"]`)
	})

	f.RunLaunch("a fact survives a kill right after the response", func(s *extensions.Launch) {
		s.GivenGitHubSees("acme/api").
			GivenEarlierStartKilled(connect("acme/api")).
			Covers("STORE-1.2.1").
			When(spec.Get("/repositories")).
			Then("the repository is listed").
			ExpectStatus(http.StatusOK).
			ExpectElement(repository("acme/api"))
	})

	f.RunLaunch("a journal file removed while gc runs stops gc", func(s *extensions.Launch) {
		s.GivenGitHubSees("acme/api").
			Covers("STORE-1.2.3").
			WhenJournalRemovedBefore(connect("acme/api")).
			Then("gc exits without confirming the connection").
			ExpectExit().
			ExpectOutput("cannot write to the journal at " + s.JournalPath())
	})

	f.RunLaunch("a journal file replaced while gc runs stops gc", func(s *extensions.Launch) {
		s.GivenGitHubSees("acme/api").
			Covers("STORE-1.2.3").
			WhenJournalReplacedBefore(connect("acme/api")).
			Then("gc exits without confirming the connection").
			ExpectExit().
			ExpectOutput("cannot write to the journal at " + s.JournalPath())
	})

	f.RunLaunch("a fact that cannot be written stops gc", func(s *extensions.Launch) {
		s.GivenGitHubSees("acme/api").
			GivenJournal(contract.RepositoryConnected{FullName: "acme/web"}).
			Covers("STORE-1.2.2").
			WhenWriteFails(connect("acme/api")).
			Then("gc exits without confirming the connection, and the file has no new fact").
			ExpectExit().
			ExpectOutput("cannot write to the journal at " + s.JournalPath()).
			ExpectJournal(contract.RepositoryConnected{FullName: "acme/web"})
	})

	f.Run("a start makes up no missed wake-ups", func(s *extensions.Scenario) {
		s.Covers("STORE-1.3.1").
			Given(connected).
			WhenRunning("10:30", time.Minute).
			Then("nothing is admitted until the next hour").
			ExpectFacts().
			Expect(s.AgentAsked())
	})

	f.Run("a run that a restart cut off is closed", func(s *extensions.Scenario) {
		s.Covers("STORE-2.1.1", "STORE-2.1.2").
			Given(connected, running).
			WhenRunning("10:30", time.Minute).
			Then("it is interrupted at the time of this start, and the agent is not asked").
			ExpectFacts(contract.AgentRunFinished{Run: 1, Repository: "acme/api", Outcome: contract.OutcomeInterrupted, At: extensions.At("2000-01-01 10:30")}).
			Expect(s.AgentAsked())
	})

	f.Run("only running runs are closed", func(s *extensions.Scenario) {
		s.Covers("STORE-2.1.1").
			Given(
				contract.AgentRunStarted{Run: 1, Repository: "acme/api", At: started},
				contract.AgentRunFinished{Run: 1, Repository: "acme/api", Outcome: contract.OutcomeNoPR, At: started},
				contract.AgentRunStarted{Run: 2, Repository: "acme/api", At: started},
				contract.AgentRunStarted{Run: 3, Repository: "acme/web", At: started},
			).
			WhenRunning("10:30", time.Minute).
			Then("runs 2 and 3 are interrupted, and run 1 keeps its outcome").
			ExpectFacts(
				contract.AgentRunFinished{Run: 2, Repository: "acme/api", Outcome: contract.OutcomeInterrupted, At: extensions.At("2000-01-01 10:30")},
				contract.AgentRunFinished{Run: 3, Repository: "acme/web", Outcome: contract.OutcomeInterrupted, At: extensions.At("2000-01-01 10:30")},
			)
	})

	f.Run("a start with no running runs writes nothing", func(s *extensions.Scenario) {
		s.Covers("STORE-2.1.3").
			Given(running, contract.AgentRunFinished{Run: 1, Repository: "acme/api", Outcome: contract.OutcomeFailed, At: started}).
			WhenRunning("10:30", time.Minute).
			Then("the journal stays as it was").
			ExpectFacts()
	})

	f.RunLaunch("a start that cannot listen writes no facts", func(s *extensions.Launch) {
		s.GivenJournal(running).
			GivenAddressTaken().
			Covers("STORE-2.1.4").
			WhenStarted().
			Then("gc refuses to start, and the run stays running").
			ExpectExit().
			ExpectOutput("cannot listen on " + s.Address()).
			ExpectJournal(running)
	})

	f.RunLaunch("a start with an unreadable journal writes no facts", func(s *extensions.Launch) {
		s.GivenJournal(running).
			GivenJournalRow("SomethingNew", `{}`).
			Covers("STORE-2.1.4", "STORE-3.2.1").
			WhenStarted().
			Then("gc refuses to start, and the file keeps its two facts").
			ExpectExit().
			ExpectOutput("cannot read the journal at " + s.JournalPath()).
			ExpectJournalRows(2)
	})

	f.RunLaunch("a fact that cannot be parsed stops the start", func(s *extensions.Launch) {
		s.GivenJournalRow("RepositoryConnected", `not json`).
			Covers("STORE-3.2.1").
			WhenStarted().
			Then("gc refuses to start").
			ExpectExit().
			ExpectOutput("cannot read the journal at " + s.JournalPath()).
			ExpectJournalRows(1)
	})

	for _, data := range []string{`null`, ` null `} {
		f.RunLaunch(fmt.Sprintf("a fact of %q stops the start", data), func(s *extensions.Launch) {
			s.GivenJournalRow("RepositoryConnected", data).
				Covers("STORE-3.2.1", "STORE-3.2.2").
				WhenStarted().
				Then("gc refuses to start").
				ExpectExit().
				ExpectOutput("cannot read the journal at " + s.JournalPath()).
				ExpectJournalRows(1)
		})
	}

	f.RunLaunch("a missing journal file is created", func(s *extensions.Launch) {
		s.Covers("STORE-3.1.1").
			WhenStarted().
			Then("the journal is empty and the file is private to its owner").
			ExpectServing().
			ExpectJournalFileMode(0o600).
			ExpectJournal()
	})

	f.RunLaunch("a journal file in a missing folder", func(s *extensions.Launch) {
		s.GivenEnv("GC_DB", s.PathInMissingFolder()).
			Covers("STORE-3.1.2").
			WhenStarted().
			Then("gc refuses to start").
			ExpectExit().
			ExpectOutput("cannot open the journal at " + s.PathInMissingFolder())
	})

	f.RunLaunch("a journal file of one byte is not a database", func(s *extensions.Launch) {
		s.GivenJournalFileContents("x").
			Covers("STORE-3.1.2").
			WhenStarted().
			Then("gc refuses to start and leaves the file alone").
			ExpectExit().
			ExpectOutput("cannot open the journal at " + s.JournalPath()).
			ExpectJournalFileUnchanged()
	})

	f.RunLaunch("a journal file that is not a database", func(s *extensions.Launch) {
		s.GivenJournalFileContents("the operator's notes, not a database").
			Covers("STORE-3.1.2").
			WhenStarted().
			Then("gc refuses to start and leaves the file alone").
			ExpectExit().
			ExpectOutput("cannot open the journal at " + s.JournalPath()).
			ExpectJournalFileUnchanged()
	})

	f.RunLaunch("a journal file in use by another gc", func(s *extensions.Launch) {
		s.GivenGitHubSees(extensions.OtherGCRepository).
			GivenAnotherGCRunning().
			Covers("STORE-3.1.3").
			WhenStarted().
			Then("the second gc refuses to start, and the first still writes").
			ExpectExit().
			ExpectOutput("cannot open the journal at " + s.JournalPath()).
			ExpectJournal(contract.RepositoryConnected{FullName: extensions.OtherGCRepository})
	})

	f.RunLaunch("secrets stay out of the journal file", func(s *extensions.Launch) {
		s.GivenGitHubSees("acme/api").
			GivenEarlierStart(connect("acme/api")).
			Covers("STORE-4.1.1").
			WhenStarted().
			Then("the file has the fact and none of the secrets").
			ExpectServing().
			ExpectJournal(connected).
			ExpectJournalFileWithout(s.GitHubToken()).
			ExpectJournalFileWithout(s.OpenAIKey()).
			ExpectJournalFileWithout(s.WebhookSecret())
	})

	f.RunLaunch("a token echoed by GitHub stays out of the journal file", func(s *extensions.Launch) {
		s.GivenGitHubEchoesToken().
			GivenGitHubAnswers(http.StatusInternalServerError).
			GivenEarlierStart(connect("acme/api")).
			Covers("STORE-4.1.1").
			WhenStarted().
			Then("the file has no fact and no token").
			ExpectServing().
			ExpectJournal().
			ExpectJournalFileWithout(s.GitHubToken())
	})
}
