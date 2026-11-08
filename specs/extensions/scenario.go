package extensions

import (
	"slices"
	"time"

	"github.com/Andrew-Sem/gc/framework/spec"
	"github.com/Andrew-Sem/gc/specs/contract"
)

const (
	UpdatedBlock = `#repositories-panel[hx-target="#repositories-panel"][hx-swap="outerHTML"]:has(form[hx-post="/repositories"] input[name="repo"]):has(#repositories)`
	WholePage    = `h1, script`
)

type Scenario struct {
	*spec.Scenario[contract.Fact]
	github  *fakeGitHub
	agent   *fakeAgent
	streams []*openStream
}

func At(moment string) time.Time {
	at, err := time.Parse("2006-01-02 15:04", moment)
	if err != nil {
		panic(err)
	}
	return at
}

func newScenario(core *spec.Scenario[contract.Fact]) *Scenario {
	return &Scenario{Scenario: core, github: newFakeGitHub(), agent: &fakeAgent{}}
}

func (s *Scenario) Covers(ids ...string) *Scenario {
	s.Scenario.Covers(ids...)
	return s
}

func (s *Scenario) Given(facts ...contract.Fact) *Scenario {
	s.Scenario.Given(facts...)
	return s
}

func (s *Scenario) GivenRequest(r spec.Request) *Scenario {
	s.Scenario.GivenRequest(r)
	return s
}

func (s *Scenario) GivenOpenPRs(repository string, count int) *Scenario {
	for number := 1; number <= count; number++ {
		s.Scenario.Given(contract.PrCreated{Repository: repository, Number: number})
	}
	return s
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

func (s *Scenario) GivenGitHubUnreachable() *Scenario {
	s.github.given(func(g *fakeGitHub) { g.unreachable = true })
	return s
}

func (s *Scenario) GivenGitHubMovedRepository(status int) *Scenario {
	s.github.given(func(g *fakeGitHub) { g.moves = status })
	return s
}

func (s *Scenario) GivenGitHubCreatesPR(number int) *Scenario {
	s.github.given(func(g *fakeGitHub) { g.createsPR = number })
	return s
}

func (s *Scenario) GivenGitHubCreatesPRAfter(number int, wait time.Duration) *Scenario {
	s.github.given(func(g *fakeGitHub) { g.createsPR, g.createsAfter = number, wait })
	return s
}

func (s *Scenario) GivenAgentChanges(branch, title, description string) *Scenario {
	s.agent.change = &contract.AgentChange{Branch: branch, Base: "main", Title: title, Description: description}
	return s
}

func (s *Scenario) GivenAgentFails() *Scenario {
	s.agent.fails = true
	return s
}

func (s *Scenario) GivenAgentHangs() *Scenario {
	s.agent.hangs = true
	return s
}

func (s *Scenario) GivenAgentWorksFor(duration time.Duration) *Scenario {
	s.agent.worksFor = duration
	return s
}

func (s *Scenario) AgentAsked(repositories ...string) func() spec.Problem {
	return func() spec.Problem {
		want := slices.Sorted(slices.Values(repositories))
		got := slices.Sorted(slices.Values(s.agent.askedSoFar()))
		if !slices.Equal(got, want) {
			return agentAsked{want: want, got: got}
		}
		return nil
	}
}

func (s *Scenario) GitHubPRCreated(repository, branch, title, description string) func() spec.Problem {
	return func() spec.Problem {
		want := createdPR{Repository: repository, Head: branch, Base: "main", Title: title, Description: description}
		s.github.mu.Lock()
		defer s.github.mu.Unlock()
		if !slices.Equal(s.github.created, []createdPR{want}) {
			return githubPRCreated{want: want, got: s.github.created}
		}
		return nil
	}
}

func (s *Scenario) GitHubToken() string {
	return fakeGitHubToken
}

func (s *Scenario) WebhookSecret() string {
	return fakeWebhookSecret
}

func (s *Scenario) GitHubNotAsked() func() spec.Problem {
	return s.GitHubAsked()
}

func (s *Scenario) GitHubAsked(fullNames ...string) func() spec.Problem {
	return func() spec.Problem {
		if asked := s.github.askedSoFar(); !slices.Equal(asked, fullNames) {
			return githubAsked{want: fullNames, got: asked}
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
