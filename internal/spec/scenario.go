package spec

type Scenario struct {
	request Request
	status  int
}

func (s *Scenario) When(r Request) *Scenario {
	s.request = r
	return s
}

func (s *Scenario) Then(description string) *Scenario {
	return s
}

func (s *Scenario) ExpectStatus(status int) *Scenario {
	s.status = status
	return s
}
