package journal

import "slices"

type Journal[F any] struct {
	facts []F
}

func (j *Journal[F]) Append(facts ...F) {
	j.facts = append(j.facts, facts...)
}

func (j *Journal[F]) Facts() []F {
	return slices.Clone(j.facts)
}
