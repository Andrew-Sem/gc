package specs

import (
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/Andrew-Sem/gc/framework/spec"
	"github.com/Andrew-Sem/gc/specs/contract"
	"github.com/Andrew-Sem/gc/specs/extensions"
	"github.com/Andrew-Sem/gc/ui"
)

const settingsPanel = `#settings-panel[hx-target="#settings-panel"][hx-swap="outerHTML"]:has(form[hx-post="/repositories/acme/api/settings"])`

func TestRepositorySettings(t *testing.T) {
	at := func(clock string) time.Time { return extensions.At("2000-01-01 " + clock) }
	f := extensions.NewFeature(t, "repository-settings.md")
	connected := contract.RepositoryConnected{FullName: "acme/api"}
	saved := func(pace, limit int, moment string) contract.RepositorySettingsChanged {
		return contract.RepositorySettingsChanged{Repository: "acme/api", Pace: pace, Limit: limit, At: extensions.At(moment)}
	}
	save := func(pace, limit string) spec.Request {
		return spec.Post("/repositories/acme/api/settings").WithForm("pace", pace).WithForm("limit", limit)
	}

	f.Run("each repository links to its settings", func(s *extensions.Scenario) {
		s.Covers("SET-1.1.1").
			Given(connected).
			When(spec.Get("/repositories")).
			Then("the row has a Settings link").
			ExpectStatus(http.StatusOK).
			ExpectFacts().
			ExpectText(`#repositories li[data-repository="acme/api"] a[href="/repositories/acme/api/settings"]`, "Settings")
	})

	f.Run("the settings page of a connected repository", func(s *extensions.Scenario) {
		s.Covers("SET-1.1.2", "SET-1.1.3", "SET-1.1.5").
			Given(connected).
			When(spec.Get("/repositories/acme/api/settings")).
			Then("the page shows the default settings in the form").
			ExpectStatus(http.StatusOK).
			ExpectFacts().
			ExpectElement(`script[src="`+ui.HTMXPath+`"]`).
			ExpectText(`h1`, "acme/api").
			ExpectElement(`a[href="/repositories"]`).
			ExpectElement(settingsPanel).
			ExpectElement(`#settings-panel input[name="pace"][value="1"]`).
			ExpectElement(`#settings-panel input[name="limit"][value="12"]`).
			ExpectText(`#settings-panel button`, "Save")
	})

	f.Run("the settings page shows saved settings", func(s *extensions.Scenario) {
		s.Covers("SET-1.1.3").
			Given(connected, saved(3, 5, "1999-12-31 10:00"), saved(6, 4, "1999-12-31 11:00")).
			When(spec.Get("/repositories/acme/api/settings")).
			Then("the form has the latest saved values").
			ExpectStatus(http.StatusOK).
			ExpectFacts().
			ExpectElement(`#settings-panel input[name="pace"][value="6"]`).
			ExpectElement(`#settings-panel input[name="limit"][value="4"]`)
	})

	f.Run("the settings page of a repository that is not connected", func(s *extensions.Scenario) {
		s.Covers("SET-1.1.4", "SET-1.1.5").
			Given(connected, contract.RepositoryDisconnected{FullName: "acme/api"}).
			When(spec.Get("/repositories/acme/api/settings")).
			Then("the page is not found").
			ExpectStatus(http.StatusNotFound).
			ExpectFacts().
			ExpectBodyContaining("This repository is not connected")
	})

	f.Run("save settings", func(s *extensions.Scenario) {
		s.Covers("SET-1.2.1").
			Given(connected).
			When(save("3", "5")).
			Then("the settings are saved and the block says so").
			ExpectStatus(http.StatusOK).
			ExpectFacts(saved(3, 5, "2000-01-01 00:00")).
			ExpectElement(settingsPanel).
			ExpectNoElement(extensions.WholePage).
			ExpectElement(`#settings-panel input[name="pace"][value="3"]`).
			ExpectElement(`#settings-panel input[name="limit"][value="5"]`).
			ExpectText(`#settings-panel [role="status"]`, "Saved").
			ExpectNoElement(`#settings-panel [role="alert"]`)
	})

	f.Run("save the settings the repository already has", func(s *extensions.Scenario) {
		s.Covers("SET-1.2.2").
			Given(connected, saved(3, 5, "1999-12-31 10:00")).
			When(save("3", "5")).
			Then("nothing is written, and the block says saved").
			ExpectStatus(http.StatusOK).
			ExpectFacts().
			ExpectText(`#settings-panel [role="status"]`, "Saved")
	})

	f.Run("save the default settings", func(s *extensions.Scenario) {
		s.Covers("SET-1.2.2").
			Given(connected).
			When(save("1", "12")).
			Then("nothing is written, and the block says saved").
			ExpectStatus(http.StatusOK).
			ExpectFacts().
			ExpectText(`#settings-panel [role="status"]`, "Saved")
	})

	f.Run("saving makes no decision", func(s *extensions.Scenario) {
		s.Covers("SET-1.2.3").
			Given(connected).
			When(save("1", "20")).
			Then("only the settings are written").
			ExpectStatus(http.StatusOK).
			ExpectFacts(saved(1, 20, "2000-01-01 00:00")).
			Expect(s.AgentAsked())
	})

	f.Run("save settings of a repository named in another casing", func(s *extensions.Scenario) {
		s.Covers("SET-1.2.4").
			Given(connected).
			When(spec.Post("/repositories/Acme/API/settings").WithForm("pace", "2").WithForm("limit", "12")).
			Then("the settings of acme/api are saved").
			ExpectStatus(http.StatusOK).
			ExpectFacts(saved(2, 12, "2000-01-01 00:00"))
	})

	f.Run("refuse a save from another site", func(s *extensions.Scenario) {
		s.Covers("SET-1.2.5").
			Given(connected).
			When(save("2", "12").WithHeader("Sec-Fetch-Site", "cross-site")).
			Then("the request is rejected and nothing is saved").
			ExpectStatus(http.StatusForbidden).
			ExpectFacts()
	})

	for _, pace := range []string{"169", "1.5", "two", "", "-1"} {
		f.Run("reject the pace "+quoted(pace), func(s *extensions.Scenario) {
			s.Covers("SET-1.3.1", "SET-1.3.4").
				Given(connected).
				When(save(pace, "12")).
				Then("the request is rejected and nothing is saved").
				ExpectStatus(http.StatusUnprocessableEntity).
				ExpectFacts().
				ExpectElement(settingsPanel).
				ExpectText(`#settings-panel [role="alert"]`, "Pace must be a whole number of hours from 0 to 168").
				ExpectElement(`#settings-panel input[name="pace"][value="` + pace + `"]`).
				ExpectElement(`#settings-panel input[name="limit"][value="12"]`).
				ExpectNoElement(`#settings-panel [role="status"]`)
		})
	}

	for _, limit := range []string{"0", "101", "2.5", "many", ""} {
		f.Run("reject the maximum "+quoted(limit), func(s *extensions.Scenario) {
			s.Covers("SET-1.3.2", "SET-1.3.4").
				Given(connected).
				When(save("4", limit)).
				Then("the request is rejected and nothing is saved").
				ExpectStatus(http.StatusUnprocessableEntity).
				ExpectFacts().
				ExpectText(`#settings-panel [role="alert"]`, "Maximum open must be a whole number from 1 to 100").
				ExpectElement(`#settings-panel input[name="pace"][value="4"]`).
				ExpectElement(`#settings-panel input[name="limit"][value="` + limit + `"]`)
		})
	}

	for _, tt := range []struct{ pace, limit int }{{168, 1}, {1, 100}} {
		f.Run(fmt.Sprintf("accept a pace of %d and a maximum of %d", tt.pace, tt.limit), func(s *extensions.Scenario) {
			s.Covers("SET-1.3.1", "SET-1.3.2").
				Given(connected).
				When(save(strconv.Itoa(tt.pace), strconv.Itoa(tt.limit))).
				Then("the settings are saved").
				ExpectStatus(http.StatusOK).
				ExpectFacts(saved(tt.pace, tt.limit, "2000-01-01 00:00"))
		})
	}

	f.Run("spaces around values are ignored", func(s *extensions.Scenario) {
		s.Covers("SET-1.3.3").
			Given(connected).
			When(save(" 3 ", "\t5\n")).
			Then("the settings are saved").
			ExpectStatus(http.StatusOK).
			ExpectFacts(saved(3, 5, "2000-01-01 00:00"))
	})

	f.Run("save settings of a repository that is not connected", func(s *extensions.Scenario) {
		s.Covers("SET-1.3.5").
			Given(connected, contract.RepositoryDisconnected{FullName: "acme/api"}).
			When(save("3", "5")).
			Then("the request is rejected and nothing is saved").
			ExpectStatus(http.StatusUnprocessableEntity).
			ExpectFacts().
			ExpectText(`#settings-panel [role="alert"]`, "This repository is not connected")
	})

	f.Run("a repository is decided only when due", func(s *extensions.Scenario) {
		s.Covers("SET-2.1.1", "SET-2.1.2").
			Given(connected, saved(3, 12, "2000-01-01 14:37")).
			WhenRunning("15:59", 2*time.Hour+2*time.Minute).
			Then("17:00 decides, 16:00 and 18:00 do not").
			ExpectFacts(
				contract.AdmissionApproved{Repository: "acme/api"},
				contract.AgentRunStarted{Run: 1, Repository: "acme/api", At: at("17:00")},
				contract.AgentRunFinished{Run: 1, Repository: "acme/api", Outcome: contract.OutcomeNoPR, At: at("17:00")},
			)
	})

	f.Run("a new pace starts a new interval", func(s *extensions.Scenario) {
		s.Covers("SET-2.1.2").
			Given(connected, saved(2, 12, "2000-01-01 10:00"), saved(3, 12, "2000-01-01 15:10")).
			WhenRunning("15:59", 3*time.Hour+2*time.Minute).
			Then("18:00 decides, not 16:00 by the old pace").
			ExpectFacts(
				contract.AdmissionApproved{Repository: "acme/api"},
				contract.AgentRunStarted{Run: 1, Repository: "acme/api", At: at("18:00")},
				contract.AgentRunFinished{Run: 1, Repository: "acme/api", Outcome: contract.OutcomeNoPR, At: at("18:00")},
			)
	})

	f.Run("each repository follows its own pace", func(s *extensions.Scenario) {
		s.Covers("SET-2.1.1").
			Given(
				connected,
				contract.RepositoryConnected{FullName: "acme/web"},
				saved(24, 12, "1999-12-31 17:00"),
			).
			WhenRunning("15:59", 2*time.Minute).
			Then("only acme/web, at its default pace, is decided at 16:00").
			ExpectFacts(
				contract.AdmissionApproved{Repository: "acme/web"},
				contract.AgentRunStarted{Run: 1, Repository: "acme/web", At: at("16:00")},
				contract.AgentRunFinished{Run: 1, Repository: "acme/web", Outcome: contract.OutcomeNoPR, At: at("16:00")},
			)
	})

	f.Run("a run request decides when the repository is not due", func(s *extensions.Scenario) {
		s.Covers("SET-2.1.3", "SET-2.1.4").
			Given(connected, saved(3, 12, "2000-01-01 14:00")).
			GivenRunRequestedAt("acme/api", "16:30").
			WhenRunning("15:59", 3*time.Hour+2*time.Minute).
			Then("16:30 decides on request, and 17:00 still decides by the pace").
			ExpectFacts(
				contract.RunRequested{Repository: "acme/api"},
				contract.AdmissionApproved{Repository: "acme/api"},
				contract.AgentRunStarted{Run: 1, Repository: "acme/api", At: at("16:30")},
				contract.AgentRunFinished{Run: 1, Repository: "acme/api", Outcome: contract.OutcomeNoPR, At: at("16:30")},
				contract.AdmissionApproved{Repository: "acme/api"},
				contract.AgentRunStarted{Run: 2, Repository: "acme/api", At: at("17:00")},
				contract.AgentRunFinished{Run: 2, Repository: "acme/api", Outcome: contract.OutcomeNoPR, At: at("17:00")},
			)
	})

	f.Run("decisions follow the maximum open", func(s *extensions.Scenario) {
		s.Covers("SET-2.2.1", "SET-2.2.2").
			Given(connected).
			GivenOpenPRs("acme/api", 5).
			Given(saved(1, 3, "1999-12-31 10:00")).
			WhenWokenUp().
			Then("five open with a maximum of three is a decline, and nothing is closed").
			ExpectFacts(contract.AdmissionDeclined{Repository: "acme/api", Open: 5, Limit: 3}).
			Expect(s.GitHubNotAsked())
	})

	f.Run("a higher maximum makes room", func(s *extensions.Scenario) {
		s.Covers("SET-2.2.1").
			Given(connected).
			GivenOpenPRs("acme/api", 12).
			Given(saved(1, 13, "1999-12-31 10:00")).
			WhenWokenUp().
			Then("twelve open with a maximum of thirteen is an approval").
			ExpectFacts(contract.AdmissionApproved{Repository: "acme/api"})
	})

	f.Run("settings survive a reconnection", func(s *extensions.Scenario) {
		s.Covers("SET-2.3.1").
			Given(
				connected,
				saved(1, 2, "1999-12-31 10:00"),
				contract.RepositoryDisconnected{FullName: "acme/api"},
				connected,
			).
			GivenOpenPRs("acme/api", 2).
			WhenWokenUp().
			Then("the saved maximum still applies").
			ExpectFacts(contract.AdmissionDeclined{Repository: "acme/api", Open: 2, Limit: 2})
	})

	f.Run("save a pace of 0", func(s *extensions.Scenario) {
		s.Covers("SET-1.2.1", "SET-1.3.1").
			Given(connected).
			When(save("0", "12")).
			Then("the pace of 0 is saved").
			ExpectStatus(http.StatusOK).
			ExpectFacts(saved(0, 12, "2000-01-01 00:00")).
			ExpectElement(`#settings-panel input[name="pace"][value="0"]`).
			ExpectText(`#settings-panel [role="status"]`, "Saved")
	})

	f.Run("a pace of 0 gets no decision at a wake-up", func(s *extensions.Scenario) {
		s.Covers("SET-2.1.5").
			Given(connected, saved(0, 12, "2000-01-01 14:37")).
			WhenRunning("14:59", 25*time.Hour+2*time.Minute).
			Then("no wake-up in a day decides").
			ExpectFacts()
	})

	f.Run("a pace of 0 still decides on a run request", func(s *extensions.Scenario) {
		s.Covers("SET-2.1.5").
			Given(connected, saved(0, 12, "2000-01-01 14:00")).
			GivenRunRequestedAt("acme/api", "16:30").
			WhenRunning("15:59", 2*time.Hour+2*time.Minute).
			Then("16:30 decides on request, and no wake-up does").
			ExpectFacts(
				contract.RunRequested{Repository: "acme/api"},
				contract.AdmissionApproved{Repository: "acme/api"},
				contract.AgentRunStarted{Run: 1, Repository: "acme/api", At: at("16:30")},
				contract.AgentRunFinished{Run: 1, Repository: "acme/api", Outcome: contract.OutcomeNoPR, At: at("16:30")},
			)
	})

	f.Run("the list shows a pace of 0", func(s *extensions.Scenario) {
		s.Covers("SET-2.3.3").
			Given(connected, contract.RepositoryConnected{FullName: "acme/web"}, saved(0, 3, "1999-12-31 10:00")).
			When(spec.Get("/repositories")).
			Then("acme/api runs only on request, acme/web every hour").
			ExpectStatus(http.StatusOK).
			ExpectFacts().
			ExpectText(`#repositories li[data-repository="acme/api"]`, "acme/api only on Run now, at most 3 open Run now Sweep now Disconnect Settings").
			ExpectText(`#repositories li[data-repository="acme/web"]`, "acme/web every 1 h, at most 12 open Run now Sweep now Disconnect Settings")
	})

	inDescription := contract.RepositorySettingsChanged{Repository: "acme/api", Pace: 1, Limit: 12, ReportInDescription: true, At: extensions.At("1999-12-31 10:00")}

	f.Run("the report goes to a file by default", func(s *extensions.Scenario) {
		s.Covers("SET-2.4.1").
			Given(connected).
			When(spec.Get("/repositories/acme/api/settings")).
			Then("the checkbox is there and not checked").
			ExpectStatus(http.StatusOK).
			ExpectFacts().
			ExpectText(`#settings-panel label:has(input[type="checkbox"][name="report_in_description"])`, "Put the report in the PR description, not in a file").
			ExpectNoElement(`#settings-panel input[name="report_in_description"][checked]`)
	})

	f.Run("the settings form shows the report in the description", func(s *extensions.Scenario) {
		s.Covers("SET-2.4.1").
			Given(connected, inDescription).
			When(spec.Get("/repositories/acme/api/settings")).
			Then("the checkbox is checked").
			ExpectStatus(http.StatusOK).
			ExpectFacts().
			ExpectElement(`#settings-panel input[type="checkbox"][name="report_in_description"][checked]`)
	})

	f.Run("put the report in the description", func(s *extensions.Scenario) {
		s.Covers("SET-2.4.1", "SET-2.4.2").
			Given(connected).
			When(save("1", "12").WithForm("report_in_description", "on")).
			Then("it is saved, and the form shows it on").
			ExpectStatus(http.StatusOK).
			ExpectFacts(contract.RepositorySettingsChanged{Repository: "acme/api", Pace: 1, Limit: 12, ReportInDescription: true, At: at("00:00")}).
			ExpectElement(`#settings-panel input[name="report_in_description"][checked]`)
	})

	f.Run("put the report back in a file", func(s *extensions.Scenario) {
		s.Covers("SET-2.4.2").
			Given(connected, inDescription).
			When(save("1", "12")).
			Then("the file is saved again").
			ExpectStatus(http.StatusOK).
			ExpectFacts(saved(1, 12, "2000-01-01 00:00")).
			ExpectNoElement(`#settings-panel input[name="report_in_description"][checked]`)
	})

	f.Run("save the report in the description again", func(s *extensions.Scenario) {
		s.Covers("SET-2.4.2").
			Given(connected, inDescription).
			When(save("1", "12").WithForm("report_in_description", "on")).
			Then("nothing is written").
			ExpectStatus(http.StatusOK).
			ExpectFacts()
	})

	f.Run("an approval carries the report setting to the agent", func(s *extensions.Scenario) {
		s.Covers("SET-2.4.3").
			Given(connected, inDescription).
			WhenWokenUpFor(time.Minute).
			Then("the approval and the agent's task say the report goes in the description").
			ExpectFacts(
				contract.AdmissionApproved{Repository: "acme/api", ReportInDescription: true},
				contract.AgentRunStarted{Run: 1, Repository: "acme/api", At: at("00:00")},
				contract.AgentRunFinished{Run: 1, Repository: "acme/api", Outcome: contract.OutcomeNoPR, At: at("00:00")},
			).
			Expect(s.AgentGotTasks(contract.AgentTask{Repository: "acme/api", ReportInDescription: true}))
	})

	f.Run("a report setting saved during a run waits for the next decision", func(s *extensions.Scenario) {
		changed := contract.RepositorySettingsChanged{Repository: "acme/api", Pace: 1, Limit: 12, At: at("15:10")}
		s.Covers("SET-2.4.3").
			Given(connected, inDescription).
			GivenAgentWorksFor(30*time.Minute).
			GivenFactWrittenAt("15:10", changed).
			WhenRunning("14:59", time.Hour+32*time.Minute).
			Then("the 15:00 run keeps the description, and the 16:00 one gets the file").
			ExpectFacts(
				contract.AdmissionApproved{Repository: "acme/api", ReportInDescription: true},
				contract.AgentRunStarted{Run: 1, Repository: "acme/api", At: at("15:00")},
				changed,
				contract.AgentRunFinished{Run: 1, Repository: "acme/api", Outcome: contract.OutcomeNoPR, At: at("15:30")},
				contract.AdmissionApproved{Repository: "acme/api"},
				contract.AgentRunStarted{Run: 2, Repository: "acme/api", At: at("16:00")},
				contract.AgentRunFinished{Run: 2, Repository: "acme/api", Outcome: contract.OutcomeNoPR, At: at("16:30")},
			).
			Expect(s.AgentGotTasks(
				contract.AgentTask{Repository: "acme/api", ReportInDescription: true},
				contract.AgentTask{Repository: "acme/api"},
			))
	})

	f.Run("the list shows each repository's settings", func(s *extensions.Scenario) {
		s.Covers("SET-2.3.2").
			Given(connected, contract.RepositoryConnected{FullName: "acme/web"}, saved(24, 3, "1999-12-31 10:00")).
			When(spec.Get("/repositories")).
			Then("acme/api has its own settings, acme/web the defaults").
			ExpectStatus(http.StatusOK).
			ExpectFacts().
			ExpectText(`#repositories li[data-repository="acme/api"]`, "acme/api every 24 h, at most 3 open Run now Sweep now Disconnect Settings").
			ExpectText(`#repositories li[data-repository="acme/web"]`, "acme/web every 1 h, at most 12 open Run now Sweep now Disconnect Settings")
	})
}

func quoted(value string) string {
	return `"` + value + `"`
}
