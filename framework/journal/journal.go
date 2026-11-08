package journal

import (
	"slices"
	"sync"
)

type Journal[F any] struct {
	mu          sync.Mutex
	facts       []F
	subscribers map[chan struct{}]struct{}
}

func (j *Journal[F]) Append(facts ...F) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.facts = append(j.facts, facts...)
	for subscriber := range j.subscribers {
		select {
		case subscriber <- struct{}{}:
		default:
		}
	}
}

func (j *Journal[F]) Facts() []F {
	j.mu.Lock()
	defer j.mu.Unlock()
	return slices.Clone(j.facts)
}

func (j *Journal[F]) Subscribe() (<-chan struct{}, func()) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.subscribers == nil {
		j.subscribers = map[chan struct{}]struct{}{}
	}
	changes := make(chan struct{}, 1)
	j.subscribers[changes] = struct{}{}
	return changes, func() {
		j.mu.Lock()
		defer j.mu.Unlock()
		delete(j.subscribers, changes)
	}
}
