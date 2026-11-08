package specs

import (
	"net/http"
	"testing"

	"github.com/Andrew-Sem/gc/framework/spec"
	"github.com/Andrew-Sem/gc/specs/contract"
	"github.com/Andrew-Sem/gc/specs/extensions"
	"github.com/Andrew-Sem/gc/ui"
)

func pullCell(pull, name string) string {
	return `#pulls tr[data-pull="` + pull + `"] td[data-cell="` + name + `"]`
}

func TestPullRequestsPage(t *testing.T) {
	f := extensions.NewFeature(t, "pull-requests-page.md")
	connected := contract.RepositoryConnected{FullName: "acme/api"}
	created := contract.PrCreated{Repository: "acme/api", Number: 7}
	mergeOn := contract.RepositorySettingsChanged{Repository: "acme/api", Pace: 1, Limit: 12, Merge: true, At: extensions.At("1999-12-31 09:00")}
	mergeOff := contract.RepositorySettingsChanged{Repository: "acme/api", Pace: 1, Limit: 12, At: extensions.At("1999-12-31 10:00")}
	closed := extensions.OpenPullOnGitHub()
	closed.Closed = true

	f.Run("an open PR is on the page", func(s *extensions.Scenario) {
		s.Covers("PULLS-1.1.1", "PULLS-1.1.6", "PULLS-1.2.1", "PULLS-1.2.2").
			Given(connected, created).
			When(spec.Get("/pulls")).
			Then("its row shows the repository and links to the PR").
			ExpectStatus(http.StatusOK).
			ExpectFacts().
			ExpectElementCount(`#pulls tr[data-pull]`, 1).
			ExpectText(pullCell("acme/api#7", "repository"), "acme/api").
			ExpectText(pullCell("acme/api#7", "number")+` a[href="https://github.com/acme/api/pull/7"]`, "#7")
	})

	f.Run("closed PRs and PRs of disconnected repositories have no row", func(s *extensions.Scenario) {
		s.Covers("PULLS-1.1.2").
			Given(
				connected, created,
				contract.PrCreated{Repository: "acme/api", Number: 8},
				closed.Snapshot("acme/api", 8),
				contract.RepositoryConnected{FullName: "acme/web"},
				contract.PrCreated{Repository: "acme/web", Number: 9},
				contract.RepositoryDisconnected{FullName: "acme/web"},
			).
			When(spec.Get("/pulls")).
			Then("only acme/api#7 is listed").
			ExpectStatus(http.StatusOK).
			ExpectFacts().
			ExpectElementCount(`#pulls tr[data-pull]`, 1).
			ExpectElement(`#pulls tr[data-pull="acme/api#7"]`)
	})

	f.Run("the newest PR comes first", func(s *extensions.Scenario) {
		s.Covers("PULLS-1.1.3").
			Given(
				connected, contract.RepositoryConnected{FullName: "acme/web"},
				contract.PrCreated{Repository: "acme/api", Number: 3},
				contract.PrCreated{Repository: "acme/web", Number: 1},
				contract.PrCreated{Repository: "acme/api", Number: 2},
			).
			When(spec.Get("/pulls")).
			Then("the rows go acme/api#2, acme/web#1, acme/api#3").
			ExpectStatus(http.StatusOK).
			ExpectFacts().
			ExpectElement(`#pulls tr[data-pull="acme/api#2"]:first-of-type`).
			ExpectElement(`#pulls tr[data-pull="acme/api#2"] + tr[data-pull="acme/web#1"] + tr[data-pull="acme/api#3"]`)
	})

	f.Run("no open PRs", func(s *extensions.Scenario) {
		s.Covers("PULLS-1.1.4", "PULLS-1.1.6").
			Given(connected).
			When(spec.Get("/pulls")).
			Then("the page says so and has no rows").
			ExpectStatus(http.StatusOK).
			ExpectFacts().
			ExpectBodyContaining("No open PRs").
			ExpectElementCount(`#pulls tr[data-pull]`, 0)
	})

	f.Run("the page uses the shared interface head", func(s *extensions.Scenario) {
		s.Covers("PULLS-1.1.5").
			When(spec.Get("/pulls")).
			Then("htmx comes from ui").
			ExpectStatus(http.StatusOK).
			ExpectFacts().
			ExpectElement(`script[src="` + ui.HTMXPath + `"]`)
	})

	for _, tt := range []struct {
		name     string
		clause   string
		facts    []contract.Fact
		decision string
		reason   string
	}{
		{"merging is off", "PULLS-1.3.1", nil, "merging is off", ""},
		{"merging was turned off after a decline", "PULLS-1.3.1",
			[]contract.Fact{mergeOn, contract.MergeDeclined{Repository: "acme/api", Number: 7, Reason: "conflicts"}, mergeOff}, "merging is off", ""},
		{"no sweep yet", "PULLS-1.3.2", []contract.Fact{mergeOn}, "no sweep yet", ""},
		{"a declined PR", "PULLS-1.3.3",
			[]contract.Fact{mergeOn, contract.MergeDeclined{Repository: "acme/api", Number: 7, Reason: "conflicts"}}, "not merged", "conflicts"},
		{"a PR whose merge failed", "PULLS-1.3.4",
			[]contract.Fact{mergeOn, contract.MergeApproved{Repository: "acme/api", Number: 7, HeadSHA: "abc123"}, contract.MergeFailed{Repository: "acme/api", Number: 7, Reason: "GitHub answered 405"}}, "merge failed", "GitHub answered 405"},
		{"an approved PR", "PULLS-1.3.5",
			[]contract.Fact{mergeOn, contract.MergeApproved{Repository: "acme/api", Number: 7, HeadSHA: "abc123"}}, "merging", ""},
		{"a PR declined, then approved", "PULLS-1.3.6",
			[]contract.Fact{mergeOn, contract.MergeDeclined{Repository: "acme/api", Number: 7, Reason: "GitHub is still checking"}, contract.MergeApproved{Repository: "acme/api", Number: 7, HeadSHA: "abc123"}}, "merging", ""},
		{"a PR approved, then declined", "PULLS-1.3.6",
			[]contract.Fact{mergeOn, contract.MergeApproved{Repository: "acme/api", Number: 7, HeadSHA: "abc123"}, contract.MergeFailed{Repository: "acme/api", Number: 7, Reason: "GitHub did not answer"}, contract.MergeDeclined{Repository: "acme/api", Number: 7, Reason: "behind the base branch"}}, "not merged", "behind the base branch"},
	} {
		f.Run("the decision for "+tt.name, func(s *extensions.Scenario) {
			s.Covers(tt.clause).
				Given(append([]contract.Fact{connected, created}, tt.facts...)...).
				When(spec.Get("/pulls")).
				Then("the row shows the decision and its reason").
				ExpectStatus(http.StatusOK).
				ExpectFacts().
				ExpectText(pullCell("acme/api#7", "decision"), tt.decision).
				ExpectText(pullCell("acme/api#7", "reason"), tt.reason)
		})
	}

	f.Run("a decision of another PR does not count", func(s *extensions.Scenario) {
		s.Covers("PULLS-1.3.6").
			Given(connected, created, mergeOn, contract.MergeDeclined{Repository: "acme/web", Number: 7, Reason: "conflicts"}, contract.MergeDeclined{Repository: "acme/api", Number: 8, Reason: "draft"}).
			When(spec.Get("/pulls")).
			Then("acme/api#7 has no sweep yet").
			ExpectStatus(http.StatusOK).
			ExpectFacts().
			ExpectText(pullCell("acme/api#7", "decision"), "no sweep yet")
	})

	f.Run("markup in a reason is shown, not run", func(s *extensions.Scenario) {
		s.Covers("PULLS-1.3.7").
			Given(connected, created, mergeOn, contract.MergeDeclined{Repository: "acme/api", Number: 7, Reason: "<b>bold</b>"}).
			When(spec.Get("/pulls")).
			Then("the markup is text").
			ExpectStatus(http.StatusOK).
			ExpectFacts().
			ExpectText(pullCell("acme/api#7", "reason"), "<b>bold</b>").
			ExpectNoElement(pullCell("acme/api#7", "reason") + " b")
	})

	for _, page := range []string{"/repositories", "/runs"} {
		f.Run(page+" links to the pull requests page", func(s *extensions.Scenario) {
			s.Covers("PULLS-2.1.1").
				When(spec.Get(page)).
				Then("there is a Pull requests link").
				ExpectStatus(http.StatusOK).
				ExpectFacts().
				ExpectText(`a[href="/pulls"]`, "Pull requests")
		})
	}

	f.Run("the pull requests page links back", func(s *extensions.Scenario) {
		s.Covers("PULLS-2.1.2").
			When(spec.Get("/pulls")).
			Then("it links to the repositories and runs pages").
			ExpectStatus(http.StatusOK).
			ExpectFacts().
			ExpectElement(`a[href="/repositories"]`).
			ExpectElement(`a[href="/runs"]`)
	})
}
