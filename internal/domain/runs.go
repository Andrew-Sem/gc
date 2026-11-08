package domain

import (
	"slices"
	"time"

	"github.com/Andrew-Sem/gc/specs/contract"
)

type Run struct {
	Number      int
	Repository  string
	Started     time.Time
	Outcome     contract.Outcome
	PR          int
	Explanation string
	Tokens      contract.TokenUsage
	Ended       time.Time
}

func (r Run) Running() bool {
	return r.Outcome == ""
}

func (r Run) Duration(now time.Time) time.Duration {
	if r.Running() {
		return now.Sub(r.Started)
	}
	return r.Ended.Sub(r.Started)
}

func NextRunNumber(facts []contract.Fact) int {
	return len(Runs(facts)) + 1
}

func Runs(facts []contract.Fact) []Run {
	var runs []Run
	for _, f := range facts {
		switch f := f.(type) {
		case contract.AgentRunStarted:
			runs = append(runs, Run{Number: f.Run, Repository: f.Repository, Started: f.At})
		case contract.AgentRunFinished:
			if i := slices.IndexFunc(runs, func(r Run) bool { return r.Number == f.Run }); i >= 0 {
				runs[i].Outcome, runs[i].PR, runs[i].Explanation, runs[i].Tokens, runs[i].Ended = f.Outcome, f.Number, f.Explanation, f.Tokens, f.At
			}
		}
	}
	slices.SortFunc(runs, func(a, b Run) int { return b.Number - a.Number })
	return runs
}
