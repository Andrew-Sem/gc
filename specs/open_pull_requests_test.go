package specs

import (
	"testing"

	"github.com/Andrew-Sem/gc/specs/contract"
	"github.com/Andrew-Sem/gc/specs/extensions"
)

func TestOpenPullRequests(t *testing.T) {
	f := extensions.NewFeature(t, "open-pull-requests.md")
	f.Run("approve a PR when there is room", func(s *extensions.Scenario) {
		s.Covers("OPEN-1.2.1").
			Given(contract.RepositoryConnected{FullName: "acme/api"}).
			WhenWokenUp().
			Then("a new PR is approved").
			ExpectFacts(contract.AdmissionApproved{Repository: "acme/api"})
	})
}
