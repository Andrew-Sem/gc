package specs

import (
	"net/http"
	"testing"

	"github.com/Andrew-Sem/gc/internal/spec"
)

func TestConnectRepository(t *testing.T) {
	spec.Run(t, "page opens", func(s *spec.Scenario) {
		s.When(spec.Get("/repositories")).
			Then("the operator opens the repositories page").
			ExpectStatus(http.StatusOK)
	})
}
