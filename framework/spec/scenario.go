package spec

import "github.com/Andrew-Sem/gc/framework/journal"

type textExpectation struct {
	selector, text string
}

type countExpectation struct {
	selector string
	count    int
}

type Scenario[F comparable] struct {
	request    Request
	action     func(*journal.Journal[F])
	status     int
	given      []F
	requests   []Request
	expected   []F
	elements   []string
	noElements []string
	texts      []textExpectation
	counts     []countExpectation
	absentText []string
	bodyTexts  []string
	covers     []string
	checks     []func() Problem

	whenCalls   int
	statusCalls int
	factsCalls  int
}

func (s *Scenario[F]) Covers(ids ...string) *Scenario[F] {
	s.covers = ids
	return s
}

func (s *Scenario[F]) Given(facts ...F) *Scenario[F] {
	s.given = append(s.given, facts...)
	return s
}

func (s *Scenario[F]) GivenRequest(r Request) *Scenario[F] {
	s.requests = append(s.requests, r)
	return s
}

func (s *Scenario[F]) When(r Request) *Scenario[F] {
	s.request = r
	s.whenCalls++
	return s
}

func (s *Scenario[F]) WhenAction(action func(*journal.Journal[F])) *Scenario[F] {
	s.action = action
	s.whenCalls++
	return s
}

func (s *Scenario[F]) Then(description string) *Scenario[F] {
	return s
}

func (s *Scenario[F]) ExpectStatus(status int) *Scenario[F] {
	s.status = status
	s.statusCalls++
	return s
}

func (s *Scenario[F]) ExpectFacts(facts ...F) *Scenario[F] {
	s.expected = facts
	s.factsCalls++
	return s
}

func (s *Scenario[F]) ExpectElement(selector string) *Scenario[F] {
	s.elements = append(s.elements, selector)
	return s
}

func (s *Scenario[F]) ExpectNoElement(selector string) *Scenario[F] {
	s.noElements = append(s.noElements, selector)
	return s
}

func (s *Scenario[F]) ExpectText(selector, text string) *Scenario[F] {
	s.texts = append(s.texts, textExpectation{selector: selector, text: text})
	return s
}

func (s *Scenario[F]) ExpectElementCount(selector string, count int) *Scenario[F] {
	s.counts = append(s.counts, countExpectation{selector: selector, count: count})
	return s
}

func (s *Scenario[F]) ExpectBodyContaining(text string) *Scenario[F] {
	s.bodyTexts = append(s.bodyTexts, text)
	return s
}

func (s *Scenario[F]) ExpectBodyWithout(text string) *Scenario[F] {
	s.absentText = append(s.absentText, text)
	return s
}

func (s *Scenario[F]) Expect(check func() Problem) *Scenario[F] {
	s.checks = append(s.checks, check)
	return s
}
