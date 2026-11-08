package web

import (
	"context"
	"errors"

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
	numbers := domain.OpenPRs(facts, repository)
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
			if merging {
				journal.Append(contract.MergeDeclined{Repository: repository, Number: number, Reason: "GitHub did not give the PR"})
			}
			continue
		}
		record(journal, snapshot)
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
	reason, merge := domain.MergeVerdict(domain.MergeState{Draft: snapshot.Draft, RequiredChecks: required, Mergeable: snapshot.Mergeable, MergeableState: mergeableState})
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

func mergeFailure(err error) string {
	var refused github.MergeRefused
	if errors.As(err, &refused) {
		return refused.Error()
	}
	return "GitHub did not answer"
}

func record(journal contract.Journal, snapshot contract.PrStateChanged) {
	if domain.IsNewSnapshot(journal.Facts(), snapshot) {
		journal.Append(snapshot)
	}
}
