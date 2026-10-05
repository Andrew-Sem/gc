package journal

import (
	"slices"

	"github.com/Andrew-Sem/gc/internal/domain"
)

type Journal struct {
	facts []domain.Fact
}

func (j *Journal) Append(facts ...domain.Fact) {
	j.facts = append(j.facts, facts...)
}

func (j *Journal) Facts() []domain.Fact {
	return slices.Clone(j.facts)
}
