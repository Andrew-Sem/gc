package specs

import (
	"net/http"
	"testing"

	"github.com/Andrew-Sem/gc/specs/contract"
	"github.com/Andrew-Sem/gc/specs/extensions"
)

func TestReceiveWebhooks(t *testing.T) {
	f := extensions.NewFeature(t, "receive-webhooks.md")
	f.Run("accept a change to our PR", func(s *extensions.Scenario) {
		s.Covers("HOOK-1.1.1", "HOOK-3.1.2").
			Given(contract.RepositoryConnected{FullName: "acme/api"}, contract.PrCreated{Repository: "acme/api", Number: 7}).
			When(extensions.PullRequestDelivery(extensions.PullRequest{
				Action:     "opened",
				Repository: "acme/api",
				Number:     7,
			})).
			Then("the PR state is recorded").
			ExpectStatus(http.StatusNoContent).
			ExpectFacts(contract.PrStateChanged{Repository: "acme/api", Number: 7})
	})

	f.Run("reject a delivery without a signature", func(s *extensions.Scenario) {
		s.Covers("HOOK-1.2.1", "HOOK-4.1.2").
			Given(contract.RepositoryConnected{FullName: "acme/api"}, contract.PrCreated{Repository: "acme/api", Number: 7}).
			When(extensions.PullRequestDelivery(extensions.PullRequest{
				Action:     "opened",
				Repository: "acme/api",
				Number:     7,
			}).WithoutHeader("X-Hub-Signature-256")).
			Then("the delivery is rejected and nothing is recorded").
			ExpectStatus(http.StatusUnauthorized).
			ExpectFacts().
			ExpectBodyContaining("Invalid signature").
			ExpectBodyWithout(s.WebhookSecret())
	})
	f.Run("reject a delivery with a wrong signature", func(s *extensions.Scenario) {
		s.Covers("HOOK-1.2.1", "HOOK-4.1.2").
			Given(contract.RepositoryConnected{FullName: "acme/api"}, contract.PrCreated{Repository: "acme/api", Number: 7}).
			When(extensions.PullRequestDelivery(extensions.PullRequest{
				Action:     "opened",
				Repository: "acme/api",
				Number:     7,
			}).WithHeader("X-Hub-Signature-256", "sha256=0000")).
			Then("the delivery is rejected and nothing is recorded").
			ExpectStatus(http.StatusUnauthorized).
			ExpectFacts().
			ExpectBodyContaining("Invalid signature").
			ExpectBodyWithout(s.WebhookSecret())
	})
}
