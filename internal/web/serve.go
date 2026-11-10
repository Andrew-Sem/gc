package web

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"slices"
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
	closeInterruptedRuns(deps.Journal)
	log.Printf("next wake-up at %s", nextWakeUp(time.Now()).UTC().Format(time.RFC3339))
	var runs sync.WaitGroup
	turn := newTurn()
	runs.Go(func() { wakeUpEveryHour(ctx, deps, &runs, turn) })
	runs.Go(func() { runOnRequest(ctx, deps, &runs, turn) })
	server := &http.Server{Handler: New(deps)}
	stop := context.AfterFunc(ctx, func() { server.Close() })
	defer stop()
	err := server.Serve(listener)
	cancel()
	runs.Wait()
	return err
}

func closeInterruptedRuns(journal contract.Journal) {
	runs := domain.Runs(journal.Facts())
	slices.Reverse(runs)
	for _, run := range runs {
		if run.Running() {
			journal.Append(contract.AgentRunFinished{Run: run.Number, Repository: run.Repository, Outcome: contract.OutcomeInterrupted, At: time.Now().UTC()})
		}
	}
}

func wakeUpEveryHour(ctx context.Context, deps contract.Dependencies, runs *sync.WaitGroup, turn turn) {
	planned := nextWakeUp(time.Now())
	for {
		timer := time.NewTimer(time.Until(planned))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			wakeUpAt(ctx, deps, runs, turn, planned)
		}
		planned = followingWakeUp(planned, time.Now())
	}
}

func nextWakeUp(now time.Time) time.Time {
	return now.Truncate(time.Hour).Add(time.Hour)
}

func followingWakeUp(planned, now time.Time) time.Time {
	next := planned.Add(time.Hour)
	for now.Sub(next) > time.Hour {
		next = next.Add(time.Hour)
	}
	return next
}

type turn chan struct{}

func newTurn() turn {
	return make(turn, 1)
}

func (t turn) take() {
	t <- struct{}{}
}

func (t turn) give() {
	<-t
}

func WakeUp(ctx context.Context, deps contract.Dependencies, runs *sync.WaitGroup) {
	decide(ctx, deps, runs, time.Now())
}

func wakeUpAt(ctx context.Context, deps contract.Dependencies, runs *sync.WaitGroup, turn turn, planned time.Time) {
	turn.take()
	defer turn.give()
	if planned.UTC().Hour()%sweepEvery == 0 {
		Sweep(ctx, deps)
	}
	decide(ctx, deps, runs, planned)
}

func decide(ctx context.Context, deps contract.Dependencies, runs *sync.WaitGroup, planned time.Time) {
	for _, approval := range domain.WakeUpWith(deps.Journal, planned, func(repository string) (contract.AdmissionApproved, bool) {
		return admitOneAtATime(deps, repository)
	}) {
		start(ctx, deps, runs, approval)
	}
}

func runOnRequest(ctx context.Context, deps contract.Dependencies, runs *sync.WaitGroup, turn turn) {
	changes, stop := deps.Journal.Subscribe()
	defer stop()
	seen := len(deps.Journal.Facts())
	for {
		select {
		case <-ctx.Done():
			return
		case <-changes:
		}
		facts := deps.Journal.Facts()
		for _, f := range facts[seen:] {
			switch requested := f.(type) {
			case contract.RunRequested:
				runNow(ctx, deps, runs, turn, requested.Repository)
			case contract.SweepRequested:
				sweepNow(ctx, deps, turn, requested.Repository)
			}
		}
		seen = len(facts)
	}
}

func runNow(ctx context.Context, deps contract.Dependencies, runs *sync.WaitGroup, turn turn, repository string) {
	turn.take()
	defer turn.give()
	if approval, ok := admitOneAtATime(deps, repository); ok {
		start(ctx, deps, runs, approval)
	}
}

func admitOneAtATime(deps contract.Dependencies, repository string) (contract.AdmissionApproved, bool) {
	if running := domain.WorkingRun(deps.Journal.Facts(), repository); running != 0 {
		deps.Journal.Append(contract.AdmissionDeclined{Repository: repository, Running: running})
		return contract.AdmissionApproved{}, false
	}
	return domain.Admit(deps.Journal, repository, deps.OpenAIBaseURL)
}

func sweepNow(ctx context.Context, deps contract.Dependencies, turn turn, repository string) {
	turn.take()
	defer turn.give()
	if domain.IsConnected(deps.Journal.Facts(), repository) {
		sweepRepository(ctx, deps.Journal, github.New(deps.GitHub), repository)
	}
}

func start(ctx context.Context, deps contract.Dependencies, runs *sync.WaitGroup, approval contract.AdmissionApproved) {
	number := domain.NextRunNumber(deps.Journal.Facts())
	deps.Journal.Append(contract.AgentRunStarted{Run: number, Repository: approval.Repository, At: time.Now().UTC()})
	task := contract.AgentTask{Repository: approval.Repository, OpenAIBaseURL: approval.OpenAIBaseURL, ReportInDescription: approval.ReportInDescription}
	runs.Go(func() { run(ctx, deps, number, task) })
}

func run(ctx context.Context, deps contract.Dependencies, number int, task contract.AgentTask) {
	repository := task.Repository
	limited, cancel := context.WithTimeout(ctx, runLimit)
	defer cancel()
	result, err := deps.Agent.Work(limited, task)
	finished := contract.AgentRunFinished{Run: number, Repository: repository, Tokens: result.Tokens}
	switch {
	case ctx.Err() != nil:
		return
	case errors.Is(limited.Err(), context.DeadlineExceeded):
		finished.Outcome = contract.OutcomeTimedOut
	case err != nil:
		log.Printf("agent run for %s failed: %v", repository, err)
		finished.Outcome = contract.OutcomeFailed
	case result.Change == nil:
		finished.Outcome, finished.Explanation = contract.OutcomeNoPR, result.Explanation
	default:
		client := github.New(deps.GitHub)
		number, err := client.CreatePR(ctx, repository, *result.Change)
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
		reconcile(ctx, deps.Journal, client, repository, number)
	}
	finished.At = time.Now().UTC()
	deps.Journal.Append(finished)
}
