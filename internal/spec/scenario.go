package spec

import "github.com/Andrew-Sem/gc/internal/domain"

type Scenario struct {
	request        Request
	status         int
	given          []domain.Fact
	expected       []domain.Fact
	covers         []string
	githubSees     []string
	githubNotAsked bool

	whenCalls   int
	statusCalls int
	factsCalls  int
}

func (s *Scenario) Covers(ids ...string) *Scenario {
	s.covers = ids
	return s
}

func (s *Scenario) Given(facts ...domain.Fact) *Scenario {
	s.given = facts
	return s
}

func (s *Scenario) When(r Request) *Scenario {
	s.request = r
	s.whenCalls++
	return s
}

func (s *Scenario) Then(description string) *Scenario {
	return s
}

func (s *Scenario) ExpectStatus(status int) *Scenario {
	s.status = status
	s.statusCalls++
	return s
}

func (s *Scenario) ExpectFacts(facts ...domain.Fact) *Scenario {
	s.expected = facts
	s.factsCalls++
	return s
}

func (s *Scenario) GivenGitHubSees(fullNames ...string) *Scenario {
	s.githubSees = fullNames
	return s
}

func (s *Scenario) ExpectGitHubNotAsked() *Scenario {
	s.githubNotAsked = true
	return s
}
