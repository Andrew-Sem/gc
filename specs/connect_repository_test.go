package specs

import (
	"net/http"
	"testing"

	"github.com/Andrew-Sem/gc/internal/domain"
	"github.com/Andrew-Sem/gc/internal/spec"
)

func TestConnectRepository(t *testing.T) {
	spec.Run(t, "page opens", func(s *spec.Scenario) {
		s.When(spec.Get("/repositories")).
			Then("the operator opens the repositories page").
			ExpectStatus(http.StatusOK)
	})

	spec.Run(t, "disconnect writes a fact", func(s *spec.Scenario) {
		s.Given(domain.RepositoryConnected{FullName: "acme/api"}).
			When(spec.Post("/repositories/acme/api/disconnect")).
			Then("the operator disconnects a connected repository").
			ExpectFacts(domain.RepositoryDisconnected{FullName: "acme/api"})
	})
}
