package spec

type textExpectation struct {
	selector, text string
}

type Scenario[F comparable] struct {
	request    Request
	status     int
	given      []F
	expected   []F
	elements   []string
	noElements []string
	texts      []textExpectation
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
	s.given = facts
	return s
}

func (s *Scenario[F]) When(r Request) *Scenario[F] {
	s.request = r
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

func (s *Scenario[F]) ExpectCheck(check func() Problem) *Scenario[F] {
	s.checks = append(s.checks, check)
	return s
}
