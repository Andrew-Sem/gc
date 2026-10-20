package extensions

import (
	"time"

	"github.com/Andrew-Sem/gc/framework/spec"
	"github.com/Andrew-Sem/gc/specs/contract"
)

const (
	UpdatedBlock = `#repositories-panel[hx-target="#repositories-panel"][hx-swap="outerHTML"]:has(form input[name="repo"]):has(#repositories)`
	WholePage    = `h1, script`
)

type Scenario struct {
	*spec.Scenario[contract.Fact]
	github *fakeGitHub
}

func newScenario(core *spec.Scenario[contract.Fact]) *Scenario {
	return &Scenario{Scenario: core, github: newFakeGitHub()}
}

func (s *Scenario) GivenGitHubSees(fullNames ...string) *Scenario {
	s.github.given(func(g *fakeGitHub) { g.sees = fullNames })
	return s
}

func (s *Scenario) GivenGitHubDoesNotAnswer() *Scenario {
	s.github.given(func(g *fakeGitHub) { g.doesNotAnswer = true })
	return s
}

func (s *Scenario) GivenGitHubRejectsToken() *Scenario {
	s.github.given(func(g *fakeGitHub) { g.rejectsToken = true })
	return s
}

func (s *Scenario) GivenGitHubAnswers(status int, header ...string) *Scenario {
	s.github.given(func(g *fakeGitHub) { g.answer = &cannedAnswer{status: status, header: header} })
	return s
}

func (s *Scenario) GivenGitHubHangs() *Scenario {
	s.github.given(func(g *fakeGitHub) { g.hangs = true })
	return s
}

func (s *Scenario) GivenGitHubEchoesToken() *Scenario {
	s.github.given(func(g *fakeGitHub) { g.echoesToken = true })
	return s
}

func (s *Scenario) GitHubToken() string {
	return fakeGitHubToken
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
		if got != want {
			return githubWaited{want: want, got: got}
		}
		return nil
	}
}
