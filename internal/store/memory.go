package store

import (
	"context"
	"slices"
	"sync"

	"github.com/Andrew-Sem/gc/internal/domain"
)

type Memory struct {
	mu    sync.Mutex
	facts []domain.Fact
}

func NewMemory() *Memory {
	return &Memory{}
}

func (m *Memory) Append(_ context.Context, facts ...domain.Fact) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.facts = append(m.facts, facts...)
	return nil
}

func (m *Memory) Facts(_ context.Context) ([]domain.Fact, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return slices.Clone(m.facts), nil
}
