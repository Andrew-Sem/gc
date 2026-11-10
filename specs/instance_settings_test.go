package specs

import (
	"net/http"
	"testing"
	"time"

	"github.com/Andrew-Sem/gc/framework/spec"
	"github.com/Andrew-Sem/gc/specs/contract"
	"github.com/Andrew-Sem/gc/specs/extensions"
	"github.com/Andrew-Sem/gc/ui"
)

const instancePanel = `#instance-settings-panel[hx-target="#instance-settings-panel"][hx-swap="outerHTML"]:has(form[hx-post="/settings"])`

func baseURLField(value string) string {
	return `#instance-settings-panel input[name="openai_base_url"][value="` + value + `"]`
}

func TestInstanceSettings(t *testing.T) {
	at := func(clock string) time.Time { return extensions.At("2000-01-01 " + clock) }
	f := extensions.NewFeature(t, "instance-settings.md")
	proxy := "http://127.0.0.1:8317/v1"
	saved := func(url string) contract.SettingsChanged {
		return contract.SettingsChanged{OpenAIBaseURL: url, At: at("00:00")}
	}
	save := func(value string) spec.Request {
		return spec.Post("/settings").WithForm("openai_base_url", value)
	}

	f.Run("the repositories page links to the settings", func(s *extensions.Scenario) {
		s.Covers("INSTANCE-1.1.1").
			When(spec.Get("/repositories")).
			Then("there is an Instance settings link").
			ExpectStatus(http.StatusOK).
			ExpectFacts().
			ExpectText(`a[href="/settings"]`, "Instance settings")
	})

	f.Run("the settings page with nothing set", func(s *extensions.Scenario) {
		s.Covers("INSTANCE-1.1.2", "INSTANCE-1.1.3", "INSTANCE-1.1.4").
			When(spec.Get("/settings")).
			Then("the field is empty, for OpenAI's own endpoint").
			ExpectStatus(http.StatusOK).
			ExpectFacts().
			ExpectElement(`script[src="`+ui.HTMXPath+`"]`).
			ExpectText(`h1`, "Settings").
			ExpectElement(`a[href="/repositories"]`).
			ExpectElement(instancePanel).
			ExpectElement(baseURLField("")).
			ExpectText(`#instance-settings-panel button`, "Save")
	})

	f.Run("the settings page shows the default from the environment", func(s *extensions.Scenario) {
		s.Covers("INSTANCE-1.1.3").
			GivenDefaultBaseURL(proxy).
			When(spec.Get("/settings")).
			Then("the field holds the default").
			ExpectStatus(http.StatusOK).
			ExpectFacts().
			ExpectElement(baseURLField(proxy))
	})

	f.Run("the settings page shows the saved value over the default", func(s *extensions.Scenario) {
		s.Covers("INSTANCE-1.1.3").
			GivenDefaultBaseURL(proxy).
			Given(saved("https://proxy.example/v1"), saved("https://other.example")).
			When(spec.Get("/settings")).
			Then("the field holds the latest saved value").
			ExpectStatus(http.StatusOK).
			ExpectFacts().
			ExpectElement(baseURLField("https://other.example"))
	})

	f.Run("save a base URL", func(s *extensions.Scenario) {
		s.Covers("INSTANCE-1.2.1").
			When(save(proxy)).
			Then("it is saved and the block says so").
			ExpectStatus(http.StatusOK).
			ExpectFacts(saved(proxy)).
			ExpectElement(instancePanel).
			ExpectNoElement(extensions.WholePage).
			ExpectElement(baseURLField(proxy)).
			ExpectText(`#instance-settings-panel [role="status"]`, "Saved").
			ExpectNoElement(`#instance-settings-panel [role="alert"]`)
	})

	for _, tt := range []struct{ submitted, normalized string }{
		{" HTTPS://Proxy.Example:8317/v1/ ", "https://proxy.example:8317/v1"},
		{"http://proxy.example/", "http://proxy.example"},
		{"http://proxy.example/a//", "http://proxy.example/a"},
		{"\thttp://Proxy.Example/V1\n", "http://proxy.example/V1"},
	} {
		f.Run("save "+quoted(tt.submitted)+" normalized", func(s *extensions.Scenario) {
			s.Covers("INSTANCE-1.2.1").
				When(save(tt.submitted)).
				Then("the normalized value is saved and shown").
				ExpectStatus(http.StatusOK).
				ExpectFacts(saved(tt.normalized)).
				ExpectElement(baseURLField(tt.normalized))
		})
	}

	f.Run("save the value already saved", func(s *extensions.Scenario) {
		s.Covers("INSTANCE-1.2.2").
			Given(saved(proxy)).
			When(save(" "+proxy+"/")).
			Then("nothing is written, and the block says saved").
			ExpectStatus(http.StatusOK).
			ExpectFacts().
			ExpectText(`#instance-settings-panel [role="status"]`, "Saved")
	})

	f.Run("save the default from the environment", func(s *extensions.Scenario) {
		s.Covers("INSTANCE-1.2.2").
			GivenDefaultBaseURL(proxy).
			When(save(proxy)).
			Then("nothing is written, and the block says saved").
			ExpectStatus(http.StatusOK).
			ExpectFacts().
			ExpectText(`#instance-settings-panel [role="status"]`, "Saved")
	})

	f.Run("save empty over a default from the environment", func(s *extensions.Scenario) {
		s.Covers("INSTANCE-1.2.3").
			GivenDefaultBaseURL(proxy).
			When(save("  ")).
			Then("the official endpoint is saved").
			ExpectStatus(http.StatusOK).
			ExpectFacts(saved("")).
			ExpectElement(baseURLField(""))
	})

	f.Run("save empty when nothing is set", func(s *extensions.Scenario) {
		s.Covers("INSTANCE-1.2.2").
			When(save("")).
			Then("nothing is written").
			ExpectStatus(http.StatusOK).
			ExpectFacts()
	})

	for _, value := range []string{
		"proxy.example/v1", "ftp://proxy.example", "http://", "http:///v1", "http://user:secret@proxy.example",
		"http://proxy.example/v1?a=1", "http://proxy.example/v1?", "http://proxy.example/v1#top", "http://proxy.example#",
	} {
		f.Run("reject the base URL "+quoted(value), func(s *extensions.Scenario) {
			s.Covers("INSTANCE-1.2.4").
				Given(saved(proxy)).
				When(save(value)).
				Then("the request is rejected and nothing is saved").
				ExpectStatus(http.StatusUnprocessableEntity).
				ExpectFacts().
				ExpectElement(instancePanel).
				ExpectText(`#instance-settings-panel [role="alert"]`, "Enter an absolute http or https URL without credentials, query, or fragment").
				ExpectElement(baseURLField(value)).
				ExpectNoElement(`#instance-settings-panel [role="status"]`)
		})
	}

	f.Run("refuse a save from another site", func(s *extensions.Scenario) {
		s.Covers("INSTANCE-1.2.5").
			When(save(proxy).WithHeader("Sec-Fetch-Site", "cross-site")).
			Then("the request is rejected and nothing is saved").
			ExpectStatus(http.StatusForbidden).
			ExpectFacts()
	})

	connected := contract.RepositoryConnected{FullName: "acme/api"}

	f.Run("an approval carries the saved value", func(s *extensions.Scenario) {
		s.Covers("INSTANCE-2.1.1", "INSTANCE-2.1.2").
			GivenDefaultBaseURL("http://env.example").
			Given(connected, saved(proxy)).
			WhenWokenUpFor(time.Minute).
			Then("the approval and the agent's task have it").
			ExpectFacts(
				contract.AdmissionApproved{Repository: "acme/api", OpenAIBaseURL: proxy},
				contract.AgentRunStarted{Run: 1, Repository: "acme/api", At: at("00:00")},
				contract.AgentRunFinished{Run: 1, Repository: "acme/api", Outcome: contract.OutcomeNoPR, At: at("00:00")},
			).
			Expect(s.AgentGotTasks(contract.AgentTask{Repository: "acme/api", OpenAIBaseURL: proxy}))
	})

	f.Run("an approval carries the default when nothing is saved", func(s *extensions.Scenario) {
		s.Covers("INSTANCE-2.1.1").
			GivenDefaultBaseURL(proxy).
			Given(connected).
			WhenWokenUpFor(time.Minute).
			Then("the approval has the default").
			ExpectFacts(
				contract.AdmissionApproved{Repository: "acme/api", OpenAIBaseURL: proxy},
				contract.AgentRunStarted{Run: 1, Repository: "acme/api", At: at("00:00")},
				contract.AgentRunFinished{Run: 1, Repository: "acme/api", Outcome: contract.OutcomeNoPR, At: at("00:00")},
			)
	})

	f.Run("a run request carries the value too", func(s *extensions.Scenario) {
		s.Covers("INSTANCE-2.1.1").
			Given(connected, saved(proxy), contract.RepositorySettingsChanged{Repository: "acme/api", Pace: 24, Limit: 12, At: at("00:00")}).
			GivenRunRequestedAt("acme/api", "10:30").
			WhenRunning("10:29", 2*time.Minute).
			Then("the approval on request has it").
			ExpectFacts(
				contract.RunRequested{Repository: "acme/api"},
				contract.AdmissionApproved{Repository: "acme/api", OpenAIBaseURL: proxy},
				contract.AgentRunStarted{Run: 1, Repository: "acme/api", At: at("10:30")},
				contract.AgentRunFinished{Run: 1, Repository: "acme/api", Outcome: contract.OutcomeNoPR, At: at("10:30")},
			)
	})

	f.Run("a change during a run waits for the next decision", func(s *extensions.Scenario) {
		changed := contract.SettingsChanged{OpenAIBaseURL: "https://other.example", At: at("15:10")}
		s.Covers("INSTANCE-2.1.2").
			Given(connected, saved(proxy)).
			GivenAgentWorksFor(30*time.Minute).
			GivenFactWrittenAt("15:10", changed).
			WhenRunning("14:59", time.Hour+32*time.Minute).
			Then("the 15:00 run keeps the old address, and the 16:00 one gets the new").
			ExpectFacts(
				contract.AdmissionApproved{Repository: "acme/api", OpenAIBaseURL: proxy},
				contract.AgentRunStarted{Run: 1, Repository: "acme/api", At: at("15:00")},
				changed,
				contract.AgentRunFinished{Run: 1, Repository: "acme/api", Outcome: contract.OutcomeNoPR, At: at("15:30")},
				contract.AdmissionApproved{Repository: "acme/api", OpenAIBaseURL: "https://other.example"},
				contract.AgentRunStarted{Run: 2, Repository: "acme/api", At: at("16:00")},
				contract.AgentRunFinished{Run: 2, Repository: "acme/api", Outcome: contract.OutcomeNoPR, At: at("16:30")},
			).
			Expect(s.AgentGotTasks(
				contract.AgentTask{Repository: "acme/api", OpenAIBaseURL: proxy},
				contract.AgentTask{Repository: "acme/api", OpenAIBaseURL: "https://other.example"},
			))
	})
}
