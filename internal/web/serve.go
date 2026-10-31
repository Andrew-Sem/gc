package web

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/Andrew-Sem/gc/internal/domain"
	"github.com/Andrew-Sem/gc/internal/github"
	"github.com/Andrew-Sem/gc/specs/contract"
)

const runLimit = 2 * time.Hour

func Serve(ctx context.Context, listener net.Listener, deps contract.Dependencies) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	log.Printf("next wake-up at %s", nextWakeUp(time.Now()).UTC().Format(time.RFC3339))
	var runs sync.WaitGroup
	runs.Go(func() { wakeUpEveryHour(ctx, deps, &runs) })
	server := &http.Server{Handler: New(deps)}
	stop := context.AfterFunc(ctx, func() { server.Close() })
	defer stop()
	err := server.Serve(listener)
	cancel()
	runs.Wait()
	return err
}

func wakeUpEveryHour(ctx context.Context, deps contract.Dependencies, runs *sync.WaitGroup) {
	for {
		now := time.Now()
		timer := time.NewTimer(nextWakeUp(now).Sub(now))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			WakeUp(ctx, deps, runs)
		}
	}
}

func nextWakeUp(now time.Time) time.Time {
	return now.Truncate(time.Hour).Add(time.Hour)
}

func WakeUp(ctx context.Context, deps contract.Dependencies, runs *sync.WaitGroup) {
	for _, repository := range domain.WakeUp(deps.Journal) {
		number := domain.NextRunNumber(deps.Journal.Facts())
		deps.Journal.Append(contract.AgentRunStarted{Run: number, Repository: repository, At: time.Now().UTC()})
		runs.Go(func() { run(ctx, deps, number, repository) })
	}
}

func run(ctx context.Context, deps contract.Dependencies, number int, repository string) {
	limited, cancel := context.WithTimeout(ctx, runLimit)
	defer cancel()
	change, err := deps.Agent.Work(limited, repository)
	finished := contract.AgentRunFinished{Run: number, Repository: repository}
	switch {
	case ctx.Err() != nil:
		return
	case errors.Is(limited.Err(), context.DeadlineExceeded):
		finished.Outcome = contract.OutcomeTimedOut
	case err != nil:
		log.Printf("agent run for %s failed: %v", repository, err)
		finished.Outcome = contract.OutcomeFailed
	case change == nil:
		finished.Outcome = contract.OutcomeNoPR
	default:
		number, err := github.New(deps.GitHub).CreatePR(ctx, repository, *change)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			log.Printf("cannot create a PR for %s: %v", repository, err)
			finished.Outcome = contract.OutcomeFailed
			break
		}
		finished.Outcome, finished.Number = contract.OutcomePR, number
		deps.Journal.Append(contract.PrCreated{Repository: repository, Number: number})
	}
	finished.At = time.Now().UTC()
	deps.Journal.Append(finished)
}
