package extensions

import (
	"time"

	"github.com/Andrew-Sem/gc/framework/spec"
	"github.com/Andrew-Sem/gc/specs/contract"
)

type Scenario struct {
	*spec.Scenario[contract.Fact]
	github *fakeGitHub
}

func newScenario(core *spec.Scenario[contract.Fact]) *Scenario {
	return &Scenario{Scenario: core, github: newFakeGitHub()}
}

func (s *Scenario) GivenGitHubSees(fullNames ...string) *Scenario {
	s.github.mu.Lock()
	defer s.github.mu.Unlock()
	s.github.sees = fullNames
	return s
}

func (s *Scenario) GivenGitHubDoesNotAnswer() *Scenario {
	s.github.mu.Lock()
	defer s.github.mu.Unlock()
	s.github.doesNotAnswer = true
	return s
}

func (s *Scenario) GivenGitHubRejectsToken() *Scenario {
	s.github.mu.Lock()
	defer s.github.mu.Unlock()
	s.github.rejectsToken = true
	return s
}

func (s *Scenario) GivenGitHubAnswers(status int, header ...string) *Scenario {
	s.github.mu.Lock()
	defer s.github.mu.Unlock()
	s.github.answer = &cannedAnswer{status: status, header: header}
	return s
}

func (s *Scenario) GivenGitHubHangs() *Scenario {
	s.github.mu.Lock()
	defer s.github.mu.Unlock()
	s.github.hangs = true
	return s
}

func (s *Scenario) GitHubNotAsked() func() spec.Problem {
	return func() spec.Problem {
		if asked := s.github.askedSoFar(); len(asked) > 0 {
			return githubAsked{got: asked}
		}
		return nil
	}
}

func (s *Scenario) GitHubWaited(want time.Duration) func() spec.Problem {
	return func() spec.Problem {
		got := s.github.waitedSoFar()
		if got < want-want/10 || got > want+want/5 {
			return githubWaited{want: want, got: got}
		}
		return nil
	}
}
