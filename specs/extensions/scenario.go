package extensions

import (
	"github.com/Andrew-Sem/gc/framework/spec"
	"github.com/Andrew-Sem/gc/specs/contract"
)

type Scenario struct {
	*spec.Scenario[contract.Fact]
	github *fakeGitHub
}

func newScenario(core *spec.Scenario[contract.Fact]) *Scenario {
	return &Scenario{Scenario: core, github: &fakeGitHub{}}
}

func (s *Scenario) GivenGitHubSees(fullNames ...string) *Scenario {
	s.github.sees = fullNames
	return s
}

func (s *Scenario) GivenGitHubDoesNotAnswer() *Scenario {
	s.github.doesNotAnswer = true
	return s
}

func (s *Scenario) GitHubNotAsked() func() spec.Problem {
	return func() spec.Problem {
		if len(s.github.asked) > 0 {
			return githubAsked{got: s.github.asked}
		}
		return nil
	}
}
