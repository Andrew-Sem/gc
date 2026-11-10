package specs

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Andrew-Sem/gc/framework/spec"
	"github.com/Andrew-Sem/gc/specs/contract"
	"github.com/Andrew-Sem/gc/specs/extensions"
)

var ourPR = []contract.Fact{
	contract.RepositoryConnected{FullName: "acme/api"},
	contract.PrCreated{Repository: "acme/api", Number: 7},
}

func delivery(change func(*extensions.PullRequest)) spec.Request {
	pr := extensions.PullRequest{
		Action:     "synchronize",
		Repository: "acme/api",
		Number:     7,
		State:      "open",
		HeadSHA:    "abc123",
		Base:       "main",
		Mergeable:  "true",
		CreatedAt:  extensions.At("1999-12-31 08:00"),
		UpdatedAt:  extensions.At("1999-12-31 10:00"),
	}
	if change != nil {
		change(&pr)
	}
	return extensions.PullRequestDelivery(pr)
}

func snapshot(change func(*contract.PrStateChanged)) contract.PrStateChanged {
	known := contract.PrStateChanged{
		Repository: "acme/api",
		Number:     7,
		HeadSHA:    "abc123",
		Base:       "main",
		Open:       true,
		Mergeable:  contract.MergeableYes,
		CreatedAt:  extensions.At("1999-12-31 08:00"),
		UpdatedAt:  extensions.At("1999-12-31 10:00"),
	}
	if change != nil {
		change(&known)
	}
	return known
}

func checkRun(change func(*extensions.CheckRunEvent)) extensions.CheckRunEvent {
	event := extensions.CheckRunEvent{
		Action:      "completed",
		Repository:  "acme/api",
		Pulls:       []int{7},
		ID:          9,
		Name:        "test",
		AppID:       15368,
		HeadSHA:     "abc123",
		Status:      "completed",
		Conclusion:  "success",
		StartedAt:   extensions.At("1999-12-31 10:00"),
		CompletedAt: extensions.At("1999-12-31 10:05"),
	}
	if change != nil {
		change(&event)
	}
	return event
}

func checkRunState(number int, event extensions.CheckRunEvent) contract.CheckRunStateChanged {
	return contract.CheckRunStateChanged{Repository: "acme/api", Number: number, CheckRun: contract.CheckRun{
		ID: event.ID, Name: event.Name, AppID: event.AppID, HeadSHA: event.HeadSHA, Status: event.Status,
		Conclusion: event.Conclusion, StartedAt: event.StartedAt, CompletedAt: event.CompletedAt,
	}}
}

func TestReceiveWebhooks(t *testing.T) {
	f := extensions.NewFeature(t, "receive-webhooks.md")
	later := extensions.At("1999-12-31 11:00")
	earlier := extensions.At("1999-12-31 09:00")

	f.Run("accept a change to our PR", func(s *extensions.Scenario) {
		s.Covers("HOOK-1.1.1", "HOOK-3.1.2", "HOOK-4.1.1").
			Given(ourPR...).
			When(delivery(nil)).
			Then("the whole snapshot is recorded, and nothing else").
			ExpectStatus(http.StatusNoContent).
			ExpectFacts(snapshot(nil))
	})

	f.Run("record that our PR was closed", func(s *extensions.Scenario) {
		s.Covers("HOOK-3.1.5").
			Given(ourPR...).
			When(delivery(func(pr *extensions.PullRequest) { pr.Action, pr.State = "closed", "closed" })).
			Then("the PR is recorded as closed").
			ExpectStatus(http.StatusNoContent).
			ExpectFacts(snapshot(func(p *contract.PrStateChanged) { p.Open = false }))
	})

	f.Run("record that our PR was merged", func(s *extensions.Scenario) {
		s.Covers("HOOK-3.1.5").
			Given(ourPR...).
			When(delivery(func(pr *extensions.PullRequest) { pr.Action, pr.State, pr.Merged = "closed", "closed", true })).
			Then("a merged PR is recorded as closed").
			ExpectStatus(http.StatusNoContent).
			ExpectFacts(snapshot(func(p *contract.PrStateChanged) { p.Open = false }))
	})

	f.Run("reject a delivery without a signature", func(s *extensions.Scenario) {
		s.Covers("HOOK-1.2.1", "HOOK-4.1.2").
			Given(ourPR...).
			When(delivery(nil).WithoutHeader("X-Hub-Signature-256")).
			Then("the delivery is rejected and nothing is recorded").
			ExpectStatus(http.StatusUnauthorized).
			ExpectFacts().
			ExpectBodyContaining("Invalid signature").
			ExpectBodyWithout(s.WebhookSecret())
	})

	f.Run("reject a delivery with a wrong signature", func(s *extensions.Scenario) {
		s.Covers("HOOK-1.2.1", "HOOK-4.1.2").
			Given(ourPR...).
			When(delivery(nil).WithHeader("X-Hub-Signature-256", "sha256=0000")).
			Then("the delivery is rejected and nothing is recorded").
			ExpectStatus(http.StatusUnauthorized).
			ExpectFacts().
			ExpectBodyContaining("Invalid signature").
			ExpectBodyWithout(s.WebhookSecret())
	})

	f.Run("a wrong signature comes before a malformed body", func(s *extensions.Scenario) {
		s.Covers("HOOK-1.2.2").
			When(extensions.RawDelivery("pull_request", "not json").WithHeader("X-Hub-Signature-256", "sha256=0000")).
			Then("the answer is about the signature, not the body").
			ExpectStatus(http.StatusUnauthorized).
			ExpectFacts()
	})

	f.Run("a wrong signature comes before a repeated delivery", func(s *extensions.Scenario) {
		s.Covers("HOOK-1.2.2").
			Given(ourPR...).
			GivenRequest(delivery(nil).WithHeader("X-GitHub-Delivery", "seen")).
			When(delivery(nil).WithHeader("X-GitHub-Delivery", "seen").WithHeader("X-Hub-Signature-256", "sha256=0000")).
			Then("the answer is about the signature, not the repeat").
			ExpectStatus(http.StatusUnauthorized).
			ExpectFacts()
	})

	for _, header := range []string{"X-GitHub-Delivery", "X-GitHub-Event"} {
		f.Run("a delivery without "+header, func(s *extensions.Scenario) {
			s.Covers("HOOK-2.1.1").
				Given(ourPR...).
				When(delivery(nil).WithoutHeader(header)).
				Then("the delivery is refused and nothing is recorded").
				ExpectStatus(http.StatusBadRequest).
				ExpectFacts()
		})
	}

	f.Run("a body that is not JSON", func(s *extensions.Scenario) {
		s.Covers("HOOK-2.1.2").
			Given(ourPR...).
			When(extensions.RawDelivery("pull_request", "not json")).
			Then("the delivery is refused and nothing is recorded").
			ExpectStatus(http.StatusBadRequest).
			ExpectFacts()
	})

	for _, field := range []string{
		"repository.full_name",
		"pull_request.number",
		"pull_request.head.sha",
		"pull_request.base.ref",
		"pull_request.draft",
		"pull_request.state",
		"pull_request.mergeable",
		"pull_request.created_at",
		"pull_request.updated_at",
	} {
		f.Run("a delivery without "+field, func(s *extensions.Scenario) {
			s.Covers("HOOK-2.1.2").
				Given(ourPR...).
				When(extensions.PullRequestDeliveryWithout(field)).
				Then("the delivery is refused and nothing is recorded").
				ExpectStatus(http.StatusBadRequest).
				ExpectFacts()
		})
	}

	f.Run("a body over the limit", func(s *extensions.Scenario) {
		s.Covers("HOOK-2.1.3").
			Given(ourPR...).
			When(extensions.RawDelivery("pull_request", strings.Repeat("x", 25<<20+1))).
			Then("the delivery is refused and nothing is recorded").
			ExpectStatus(http.StatusRequestEntityTooLarge).
			ExpectFacts()
	})

	f.Run("a body at the limit", func(s *extensions.Scenario) {
		s.Covers("HOOK-2.1.3").
			Given(ourPR...).
			When(extensions.RawDelivery("ping", strings.Repeat("x", 25<<20))).
			Then("it is read as usual").
			ExpectStatus(http.StatusNoContent).
			ExpectFacts()
	})

	f.Run("a repeated delivery with another body", func(s *extensions.Scenario) {
		s.Covers("HOOK-2.2.1", "HOOK-2.2.2").
			Given(ourPR...).
			GivenRequest(delivery(nil).WithHeader("X-GitHub-Delivery", "seen")).
			When(delivery(func(pr *extensions.PullRequest) { pr.UpdatedAt, pr.State = later, "closed" }).WithHeader("X-GitHub-Delivery", "seen")).
			Then("the repeat is accepted and nothing new is recorded").
			ExpectStatus(http.StatusNoContent).
			ExpectFacts()
	})

	f.Run("a new delivery with the same body", func(s *extensions.Scenario) {
		s.Covers("HOOK-2.2.1", "HOOK-3.3.2").
			Given(ourPR...).
			GivenRequest(delivery(nil)).
			When(delivery(nil)).
			Then("it is accepted, and the same snapshot records nothing").
			ExpectStatus(http.StatusNoContent).
			ExpectFacts()
	})

	f.Run("a forged request does not block the real delivery", func(s *extensions.Scenario) {
		s.Covers("HOOK-2.2.3").
			Given(ourPR...).
			GivenRequest(delivery(nil).WithHeader("X-GitHub-Delivery", "first").WithHeader("X-Hub-Signature-256", "sha256=0000")).
			When(delivery(nil).WithHeader("X-GitHub-Delivery", "first")).
			Then("the real delivery is recorded").
			ExpectStatus(http.StatusNoContent).
			ExpectFacts(snapshot(nil))
	})

	f.Run("a malformed delivery can be delivered again", func(s *extensions.Scenario) {
		s.Covers("HOOK-2.2.3").
			Given(ourPR...).
			GivenRequest(extensions.RawDelivery("pull_request", "not json").WithHeader("X-GitHub-Delivery", "first")).
			When(delivery(nil).WithHeader("X-GitHub-Delivery", "first")).
			Then("the second attempt is recorded").
			ExpectStatus(http.StatusNoContent).
			ExpectFacts(snapshot(nil))
	})

	for _, event := range []string{"ping", "push"} {
		f.Run("an event we do not use: "+event, func(s *extensions.Scenario) {
			s.Covers("HOOK-2.3.1").
				Given(ourPR...).
				When(extensions.RawDelivery(event, "not json")).
				Then("it is accepted and nothing is recorded").
				ExpectStatus(http.StatusNoContent).
				ExpectFacts()
		})
	}

	for _, action := range []string{"created", "rerequested", "completed"} {
		f.Run("record a check run of our PR: "+action, func(s *extensions.Scenario) {
			event := checkRun(func(e *extensions.CheckRunEvent) { e.Action = action })
			s.Covers("HOOK-5.1.1").
				Given(ourPR...).
				When(extensions.CheckRunDelivery(event)).
				Then("the check run state is recorded").
				ExpectStatus(http.StatusNoContent).
				ExpectFacts(checkRunState(7, event))
		})
	}

	f.Run("record a check run that has not finished", func(s *extensions.Scenario) {
		event := checkRun(func(e *extensions.CheckRunEvent) {
			e.Action, e.Status, e.Conclusion, e.CompletedAt = "created", "in_progress", "", time.Time{}
		})
		s.Covers("HOOK-5.1.1").
			Given(ourPR...).
			When(extensions.CheckRunDelivery(event)).
			Then("it is recorded with no conclusion and no end").
			ExpectStatus(http.StatusNoContent).
			ExpectFacts(checkRunState(7, event))
	})

	f.Run("record a check run for each of our PRs in GitHub's order", func(s *extensions.Scenario) {
		event := checkRun(func(e *extensions.CheckRunEvent) { e.Pulls = []int{8, 7}; e.Repository = "Acme/API" })
		s.Covers("HOOK-5.1.1").
			Given(append(ourPR, contract.PrCreated{Repository: "acme/api", Number: 8})...).
			When(extensions.CheckRunDelivery(event)).
			Then("PR 8 and then PR 7 get the state, under the normalized name").
			ExpectStatus(http.StatusNoContent).
			ExpectFacts(checkRunState(8, event), checkRunState(7, event))
	})

	for _, tt := range []struct {
		name  string
		given []contract.Fact
		pulls []int
		want  []int
	}{
		{"a PR listed twice", ourPR, []int{7, 7}, []int{7}},
		{"a PR that is not ours", ourPR, []int{9}, nil},
		{"no PR listed", ourPR, nil, nil},
		{"a disconnected repository", append(ourPR, contract.RepositoryDisconnected{FullName: "acme/api"}), []int{7}, nil},
	} {
		f.Run("check runs and PRs: "+tt.name, func(s *extensions.Scenario) {
			event := checkRun(func(e *extensions.CheckRunEvent) { e.Pulls = tt.pulls })
			var want []contract.Fact
			for _, number := range tt.want {
				want = append(want, checkRunState(number, event))
			}
			s.Covers("HOOK-5.1.2").
				Given(tt.given...).
				When(extensions.CheckRunDelivery(event)).
				Then("only our PRs get a state, once each").
				ExpectStatus(http.StatusNoContent).
				ExpectFacts(want...)
		})
	}

	f.Run("a check run action we do not use", func(s *extensions.Scenario) {
		s.Covers("HOOK-5.1.3").
			Given(ourPR...).
			When(extensions.CheckRunDelivery(checkRun(func(e *extensions.CheckRunEvent) { e.Action = "requested_action" }))).
			Then("it is accepted and nothing is recorded").
			ExpectStatus(http.StatusNoContent).
			ExpectFacts()
	})

	f.Run("a check run body that is not JSON", func(s *extensions.Scenario) {
		s.Covers("HOOK-5.1.4").
			Given(ourPR...).
			When(extensions.RawDelivery("check_run", "not json")).
			Then("the request is rejected and nothing is recorded").
			ExpectStatus(http.StatusBadRequest).
			ExpectFacts()
	})

	for _, field := range []string{"check_run.id", "check_run.name", "check_run.app", "check_run.head_sha", "check_run.status", "check_run.started_at"} {
		f.Run("a check run without "+field, func(s *extensions.Scenario) {
			s.Covers("HOOK-5.1.4").
				Given(ourPR...).
				When(extensions.CheckRunDeliveryWithout(checkRun(nil), field)).
				Then("the request is rejected and nothing is recorded").
				ExpectStatus(http.StatusBadRequest).
				ExpectFacts()
		})
	}

	f.Run("the same check run state again", func(s *extensions.Scenario) {
		s.Covers("HOOK-5.2.1").
			Given(append(ourPR, checkRunState(7, checkRun(nil)))...).
			When(extensions.CheckRunDelivery(checkRun(nil))).
			Then("nothing new is recorded").
			ExpectStatus(http.StatusNoContent).
			ExpectFacts()
	})

	f.Run("another state of a known check run", func(s *extensions.Scenario) {
		failed := checkRun(func(e *extensions.CheckRunEvent) { e.Conclusion = "failure" })
		s.Covers("HOOK-5.2.1").
			Given(append(ourPR, checkRunState(7, checkRun(nil)))...).
			When(extensions.CheckRunDelivery(failed)).
			Then("the new state is recorded").
			ExpectStatus(http.StatusNoContent).
			ExpectFacts(checkRunState(7, failed))
	})

	f.Run("a late running state after the check run completed", func(s *extensions.Scenario) {
		running := checkRun(func(e *extensions.CheckRunEvent) {
			e.Action, e.Status, e.Conclusion, e.CompletedAt = "created", "in_progress", "", time.Time{}
		})
		s.Covers("HOOK-5.2.2").
			Given(append(ourPR, checkRunState(7, checkRun(nil)))...).
			When(extensions.CheckRunDelivery(running)).
			Then("the completed state stays").
			ExpectStatus(http.StatusNoContent).
			ExpectFacts()
	})

	f.Run("a completed state after a running one", func(s *extensions.Scenario) {
		running := checkRun(func(e *extensions.CheckRunEvent) {
			e.Action, e.Status, e.Conclusion, e.CompletedAt = "created", "in_progress", "", time.Time{}
		})
		s.Covers("HOOK-5.2.2").
			Given(append(ourPR, checkRunState(7, running))...).
			When(extensions.CheckRunDelivery(checkRun(nil))).
			Then("the completed state is recorded").
			ExpectStatus(http.StatusNoContent).
			ExpectFacts(checkRunState(7, checkRun(nil)))
	})

	for _, action := range []string{"labeled", "assigned"} {
		f.Run("an action we do not use: "+action, func(s *extensions.Scenario) {
			s.Covers("HOOK-2.3.2").
				Given(ourPR...).
				When(delivery(func(pr *extensions.PullRequest) { pr.Action = action })).
				Then("it is accepted and nothing is recorded").
				ExpectStatus(http.StatusNoContent).
				ExpectFacts()
		})
	}

	for _, action := range []string{"opened", "reopened", "closed", "synchronize", "edited", "ready_for_review", "converted_to_draft"} {
		f.Run("an action we use: "+action, func(s *extensions.Scenario) {
			s.Covers("HOOK-2.3.2", "HOOK-3.1.1").
				Given(ourPR...).
				When(delivery(func(pr *extensions.PullRequest) { pr.Action = action })).
				Then("the snapshot is recorded").
				ExpectStatus(http.StatusNoContent).
				ExpectFacts(snapshot(nil))
		})
	}

	f.Run("a newer snapshot", func(s *extensions.Scenario) {
		s.Covers("HOOK-3.1.1").
			Given(ourPR...).
			Given(snapshot(nil)).
			When(delivery(func(pr *extensions.PullRequest) { pr.UpdatedAt, pr.HeadSHA = later, "def456" })).
			Then("it is recorded").
			ExpectStatus(http.StatusNoContent).
			ExpectFacts(snapshot(func(p *contract.PrStateChanged) { p.UpdatedAt, p.HeadSHA = later, "def456" }))
	})

	for _, tt := range []struct {
		name  string
		event func(*extensions.PullRequest)
		fact  func(*contract.PrStateChanged)
	}{
		{"head commit", func(pr *extensions.PullRequest) { pr.HeadSHA = "def456" }, func(p *contract.PrStateChanged) { p.HeadSHA = "def456" }},
		{"target branch", func(pr *extensions.PullRequest) { pr.Base = "dev" }, func(p *contract.PrStateChanged) { p.Base = "dev" }},
		{"draft", func(pr *extensions.PullRequest) { pr.Draft = true }, func(p *contract.PrStateChanged) { p.Draft = true }},
		{"state", func(pr *extensions.PullRequest) { pr.State = "closed" }, func(p *contract.PrStateChanged) { p.Open = false }},
		{"mergeable", func(pr *extensions.PullRequest) { pr.Mergeable = "false" }, func(p *contract.PrStateChanged) { p.Mergeable = contract.MergeableNo }},
		{"creation date", func(pr *extensions.PullRequest) { pr.CreatedAt = earlier }, func(p *contract.PrStateChanged) { p.CreatedAt = earlier }},
	} {
		f.Run("a snapshot of the same date with another "+tt.name, func(s *extensions.Scenario) {
			s.Covers("HOOK-3.1.1").
				Given(ourPR...).
				Given(snapshot(nil)).
				When(delivery(tt.event)).
				Then("it is recorded, because it is not older and differs").
				ExpectStatus(http.StatusNoContent).
				ExpectFacts(snapshot(tt.fact))
		})
	}

	f.Run("a snapshot of another PR is not the known one", func(s *extensions.Scenario) {
		s.Covers("HOOK-3.1.2").
			Given(ourPR...).
			Given(contract.PrCreated{Repository: "acme/api", Number: 8}).
			Given(snapshot(func(p *contract.PrStateChanged) { p.Number, p.UpdatedAt = 8, later })).
			When(delivery(nil)).
			Then("the first snapshot of this PR is recorded, though another PR's is newer").
			ExpectStatus(http.StatusNoContent).
			ExpectFacts(snapshot(nil))
	})

	f.Run("the repository name is normalized", func(s *extensions.Scenario) {
		s.Covers("HOOK-3.1.3").
			Given(ourPR...).
			When(delivery(func(pr *extensions.PullRequest) { pr.Repository = "Acme/API" })).
			Then("it matches the connected repository").
			ExpectStatus(http.StatusNoContent).
			ExpectFacts(snapshot(nil))
	})

	f.Run("GitHub does not know yet whether the PR is mergeable", func(s *extensions.Scenario) {
		s.Covers("HOOK-3.1.4").
			Given(ourPR...).
			When(delivery(func(pr *extensions.PullRequest) { pr.Mergeable = "null" })).
			Then("mergeable is recorded as unknown").
			ExpectStatus(http.StatusNoContent).
			ExpectFacts(snapshot(func(p *contract.PrStateChanged) { p.Mergeable = contract.MergeableUnknown }))
	})

	f.Run("a repository that was never connected", func(s *extensions.Scenario) {
		s.Covers("HOOK-3.2.1").
			When(delivery(nil)).
			Then("it is accepted and nothing is recorded").
			ExpectStatus(http.StatusNoContent).
			ExpectFacts()
	})

	f.Run("a repository disconnected after its PR was created", func(s *extensions.Scenario) {
		s.Covers("HOOK-3.2.1").
			Given(ourPR...).
			Given(contract.RepositoryDisconnected{FullName: "acme/api"}).
			When(delivery(nil)).
			Then("it is accepted and nothing is recorded").
			ExpectStatus(http.StatusNoContent).
			ExpectFacts()
	})

	f.Run("a PR we did not create", func(s *extensions.Scenario) {
		s.Covers("HOOK-3.2.2").
			Given(contract.RepositoryConnected{FullName: "acme/api"}, contract.PrCreated{Repository: "acme/api", Number: 8}).
			When(delivery(nil)).
			Then("it is accepted and nothing is recorded").
			ExpectStatus(http.StatusNoContent).
			ExpectFacts()
	})

	f.Run("the same number in another repository", func(s *extensions.Scenario) {
		s.Covers("HOOK-3.2.2").
			Given(contract.RepositoryConnected{FullName: "acme/api"}, contract.PrCreated{Repository: "acme/web", Number: 7}).
			When(delivery(nil)).
			Then("it is accepted and nothing is recorded").
			ExpectStatus(http.StatusNoContent).
			ExpectFacts()
	})

	f.Run("an older snapshot", func(s *extensions.Scenario) {
		s.Covers("HOOK-3.3.1").
			Given(ourPR...).
			Given(snapshot(nil)).
			When(delivery(func(pr *extensions.PullRequest) { pr.UpdatedAt, pr.State = earlier, "closed" })).
			Then("it is accepted and nothing is recorded").
			ExpectStatus(http.StatusNoContent).
			ExpectFacts()
	})

	f.Run("a snapshot older than the latest known one", func(s *extensions.Scenario) {
		s.Covers("HOOK-3.3.1").
			Given(ourPR...).
			Given(snapshot(nil), snapshot(func(p *contract.PrStateChanged) { p.UpdatedAt = later })).
			When(delivery(func(pr *extensions.PullRequest) { pr.UpdatedAt, pr.State = extensions.At("1999-12-31 10:30"), "closed" })).
			Then("it is compared with the latest, so nothing is recorded").
			ExpectStatus(http.StatusNoContent).
			ExpectFacts()
	})

	f.Run("an identical snapshot with a newer date", func(s *extensions.Scenario) {
		s.Covers("HOOK-3.3.2").
			Given(ourPR...).
			Given(snapshot(nil)).
			When(delivery(func(pr *extensions.PullRequest) { pr.UpdatedAt = later })).
			Then("it is accepted and nothing is recorded").
			ExpectStatus(http.StatusNoContent).
			ExpectFacts()
	})

	f.Run("an identical snapshot with another time zone offset", func(s *extensions.Scenario) {
		offset := time.FixedZone("", 5*60*60+30*60)
		s.Covers("HOOK-3.3.2").
			Given(ourPR...).
			Given(snapshot(nil)).
			When(delivery(func(pr *extensions.PullRequest) {
				pr.CreatedAt = snapshot(nil).CreatedAt.In(offset)
				pr.UpdatedAt = snapshot(nil).UpdatedAt.In(offset)
			})).
			Then("it is the same moment, so nothing is recorded").
			ExpectStatus(http.StatusNoContent).
			ExpectFacts()
	})

	f.Run("a signed delivery with browser headers", func(s *extensions.Scenario) {
		s.Covers("HOOK-1.1.2").
			Given(ourPR...).
			When(delivery(nil).WithHeader("Sec-Fetch-Site", "cross-site").WithHeader("Origin", "https://evil.example")).
			Then("it is handled as usual").
			ExpectStatus(http.StatusNoContent).
			ExpectFacts(snapshot(nil))
	})

	f.RunLaunch("the webhook secret stays out of the logs", func(s *extensions.Launch) {
		s.Covers("HOOK-4.1.3").
			When(delivery(nil).WithHeader("X-Hub-Signature-256", "sha256=0000")).
			Then("the delivery is rejected, and the output never has the secret").
			ExpectStatus(http.StatusUnauthorized).
			ExpectOutputWithout(s.WebhookSecret())
	})
}
