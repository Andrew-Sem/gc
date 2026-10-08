package extensions

import (
	"github.com/Andrew-Sem/gc/framework/spec"
	"github.com/Andrew-Sem/gc/specs/contract"
)

type Scenario struct {
	*spec.Scenario[contract.Fact]
	github         *fakeGitHub
	githubNotAsked bool
}

func newScenario(core *spec.Scenario[contract.Fact]) *Scenario {
	s := &Scenario{Scenario: core, github: &fakeGitHub{}}
	core.ExpectCheck(func() spec.Problem {
		if s.githubNotAsked && len(s.github.asked) > 0 {
			return githubAsked{got: s.github.asked}
		}
		return nil
	})
	return s
}

func (s *Scenario) GivenGitHubSees(fullNames ...string) *Scenario {
	s.github.sees = fullNames
	return s
}

func (s *Scenario) GivenGitHubDoesNotAnswer() *Scenario {
	s.github.doesNotAnswer = true
	return s
}

func (s *Scenario) ExpectGitHubNotAsked() *Scenario {
	s.githubNotAsked = true
	return s
}
