package extensions

import (
	"net/http"
	"testing"

	"github.com/Andrew-Sem/gc/framework/journal"
	"github.com/Andrew-Sem/gc/framework/spec"
	"github.com/Andrew-Sem/gc/internal/web"
	"github.com/Andrew-Sem/gc/specs/contract"
)

type Feature struct {
	t      *testing.T
	core   *spec.Feature[contract.Fact]
	binary string
}

func NewFeature(t *testing.T, path string) *Feature {
	return &Feature{t: t, core: spec.NewFeature[contract.Fact](t, path)}
}

func (f *Feature) Run(name string, describe func(*Scenario)) {
	var scenario *Scenario
	f.core.Run(name, func(core *spec.Scenario[contract.Fact]) {
		scenario = newScenario(core)
		describe(scenario)
	}, func(j *journal.Journal[contract.Fact]) http.Handler {
		return scenario.app(web.New)(j)
	})
}

func (s *Scenario) app(newApp func(contract.Dependencies) http.Handler) func(*journal.Journal[contract.Fact]) http.Handler {
	return func(j *journal.Journal[contract.Fact]) http.Handler {
		return newApp(contract.Dependencies{
			Journal:       j,
			GitHub:        contract.GitHub{APIURL: fakeGitHubURL, Token: fakeGitHubToken, Transport: s.github.transport()},
			WebhookSecret: fakeWebhookSecret,
		})
	}
}
