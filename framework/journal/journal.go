package journal

import (
	"slices"
	"sync"
)

type Journal[F any] struct {
	mu    sync.Mutex
	facts []F
}

func (j *Journal[F]) Append(facts ...F) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.facts = append(j.facts, facts...)
}

func (j *Journal[F]) Facts() []F {
	j.mu.Lock()
	defer j.mu.Unlock()
	return slices.Clone(j.facts)
}
