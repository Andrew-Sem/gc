package extensions

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/Andrew-Sem/gc/specs/contract"
)

type fakeAgent struct {
	mu          sync.Mutex
	asked       []string
	change      *contract.AgentChange
	explanation string
	fails       bool
	hangs       bool
	worksFor    time.Duration
}

func (a *fakeAgent) Work(ctx context.Context, repository string) (contract.AgentResult, error) {
	a.mu.Lock()
	a.asked = append(a.asked, repository)
	a.mu.Unlock()
	if a.hangs {
		<-ctx.Done()
		return contract.AgentResult{}, ctx.Err()
	}
	select {
	case <-time.After(a.worksFor):
	case <-ctx.Done():
		return contract.AgentResult{}, ctx.Err()
	}
	if a.fails {
		return contract.AgentResult{Explanation: a.explanation}, errors.New("the agent failed")
	}
	return contract.AgentResult{Change: a.change, Explanation: a.explanation}, nil
}

func (a *fakeAgent) askedSoFar() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return slices.Clone(a.asked)
}

type agentAsked struct {
	want, got []string
}

func (p agentAsked) String() string {
	return fmt.Sprintf("expected the agent to work on %v, got %v", p.want, p.got)
}
