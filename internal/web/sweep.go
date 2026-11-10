package web

import (
	"context"
	"errors"
	"slices"
	"sync"
	"time"

	"github.com/Andrew-Sem/gc/internal/domain"
	"github.com/Andrew-Sem/gc/internal/github"
	"github.com/Andrew-Sem/gc/specs/contract"
)

const sweepEvery = 6

func Sweep(ctx context.Context, deps contract.Dependencies) {
	client := github.New(deps.GitHub)
	for _, repository := range domain.ConnectedRepositories(deps.Journal.Facts()) {
		if ctx.Err() != nil {
			return
		}
		sweepRepository(ctx, deps.Journal, client, repository)
	}
}

func sweepRepository(ctx context.Context, journal contract.Journal, client *github.Client, repository string) {
	facts := journal.Facts()
	open := domain.OpenPRs(facts, repository)
	numbers := append(open, domain.UnsettledPRs(facts, repository)...)
	if len(numbers) == 0 {
		return
	}
	merging := domain.RepositorySettings(facts, repository).Merge
	token, tokenErr := client.Token(ctx, repository)
	for _, number := range numbers {
		if ctx.Err() != nil {
			return
		}
		pull, snapshot, read := readPull(ctx, client, token, tokenErr, repository, number)
		if !read {
			if merging && slices.Contains(open, number) {
				journal.Append(contract.MergeDeclined{Repository: repository, Number: number, Reason: "GitHub did not give the PR"})
			}
			continue
		}
		record(journal, snapshot)
		if snapshot.Open {
			observeChecks(ctx, journal, client, token, snapshot)
		}
		if merging && snapshot.Open {
			mergePull(ctx, journal, client, token, snapshot, pull.MergeableState)
		}
	}
}

func readPull(ctx context.Context, client *github.Client, token string, tokenErr error, repository string, number int) (github.Pull, contract.PrStateChanged, bool) {
	if tokenErr != nil {
		return github.Pull{}, contract.PrStateChanged{}, false
	}
	pull, err := client.Pull(ctx, repository, token, number)
	if err != nil {
		return github.Pull{}, contract.PrStateChanged{}, false
	}
	snapshot, complete := pull.Snapshot(repository)
	return pull, snapshot, complete && snapshot.Number == number
}

func mergePull(ctx context.Context, journal contract.Journal, client *github.Client, token string, snapshot contract.PrStateChanged, mergeableState string) {
	repository, number := snapshot.Repository, snapshot.Number
	required, err := client.RequiredChecks(ctx, repository, token, snapshot.Base)
	if !snapshot.Draft && err == nil && len(required) > 0 && snapshot.Mergeable == contract.MergeableUnknown {
		snapshot, mergeableState = waitForMergeState(ctx, journal, client, token, snapshot, mergeableState)
		if !snapshot.Open || ctx.Err() != nil {
			return
		}
	}
	reason, merge := domain.MergeVerdict(domain.MergeState{Draft: snapshot.Draft, RequiredChecks: len(required), Mergeable: snapshot.Mergeable, MergeableState: mergeableState})
	if err != nil && !snapshot.Draft {
		reason, merge = "GitHub did not give the base branch", false
	}
	if !merge {
		journal.Append(contract.MergeDeclined{Repository: repository, Number: number, Reason: reason})
		return
	}
	journal.Append(contract.MergeApproved{Repository: repository, Number: number, HeadSHA: snapshot.HeadSHA})
	if err := client.Merge(ctx, repository, token, number, snapshot.HeadSHA); err != nil {
		if ctx.Err() != nil {
			return
		}
		journal.Append(contract.MergeFailed{Repository: repository, Number: number, Reason: mergeFailure(err)})
		return
	}
	if _, after, read := readPull(ctx, client, token, nil, repository, number); read {
		record(journal, after)
	}
}

var checkingWaits = []time.Duration{2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second}

func waitForMergeState(ctx context.Context, journal contract.Journal, client *github.Client, token string, snapshot contract.PrStateChanged, mergeableState string) (contract.PrStateChanged, string) {
	for _, wait := range checkingWaits {
		select {
		case <-ctx.Done():
			return snapshot, mergeableState
		case <-time.After(wait):
		}
		pull, again, read := readPull(ctx, client, token, nil, snapshot.Repository, snapshot.Number)
		if !read {
			continue
		}
		record(journal, again)
		snapshot, mergeableState = again, pull.MergeableState
		if snapshot.Mergeable != contract.MergeableUnknown {
			break
		}
	}
	return snapshot, mergeableState
}

func mergeFailure(err error) string {
	var refused github.MergeRefused
	if errors.As(err, &refused) {
		return refused.Error()
	}
	return "GitHub did not answer"
}

var recording sync.Mutex

func record(journal contract.Journal, snapshot contract.PrStateChanged) {
	recording.Lock()
	defer recording.Unlock()
	if domain.IsNewSnapshot(journal.Facts(), snapshot) {
		journal.Append(snapshot)
	}
}

func observeChecks(ctx context.Context, journal contract.Journal, client *github.Client, token string, snapshot contract.PrStateChanged) {
	required, err := client.RequiredChecks(ctx, snapshot.Repository, token, snapshot.Base)
	if err != nil {
		return
	}
	runs, err := client.CheckRuns(ctx, snapshot.Repository, token, snapshot.HeadSHA)
	if err != nil {
		return
	}
	for _, run := range runs {
		recordCheckRun(journal, snapshot.Repository, snapshot.Number, run)
	}
	journal.Append(contract.RequiredChecksObserved{Repository: snapshot.Repository, Number: snapshot.Number, HeadSHA: snapshot.HeadSHA, Required: required})
}

func reconcile(ctx context.Context, journal contract.Journal, client *github.Client, repository string, number int) {
	token, err := client.Token(ctx, repository)
	_, snapshot, read := readPull(ctx, client, token, err, repository, number)
	if !read {
		return
	}
	record(journal, snapshot)
	if snapshot.Open {
		observeChecks(ctx, journal, client, token, snapshot)
	}
}

func recordCheckRun(journal contract.Journal, repository string, number int, run contract.CheckRun) {
	recording.Lock()
	defer recording.Unlock()
	if domain.IsNewCheckRunState(journal.Facts(), repository, number, run) {
		journal.Append(contract.CheckRunStateChanged{Repository: repository, Number: number, CheckRun: run})
	}
}
