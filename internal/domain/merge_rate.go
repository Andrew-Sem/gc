package domain

import (
	"fmt"
	"slices"

	"github.com/Andrew-Sem/gc/specs/contract"
)

type MergeRate struct {
	Merged  int
	Settled int
}

func (r MergeRate) Text() string {
	if r.Settled == 0 {
		return "No closed agent PRs yet"
	}
	percent := (r.Merged*200 + r.Settled) / (r.Settled * 2)
	return fmt.Sprintf("Merged %d of %d closed agent PRs (%d%%)", r.Merged, r.Settled, percent)
}

type pullKey struct {
	repository string
	number     int
}

func MergeRateOf(facts []contract.Fact) MergeRate {
	var rate MergeRate
	for _, state := range latestStatesOfOurs(facts) {
		if state.Open || state.Merged == "" {
			continue
		}
		rate.Settled++
		if state.Merged == contract.MergedYes {
			rate.Merged++
		}
	}
	return rate
}

func UnsettledPRs(facts []contract.Fact, repository string) []int {
	var numbers []int
	for key, state := range latestStatesOfOurs(facts) {
		if key.repository == repository && !state.Open && state.Merged == "" {
			numbers = append(numbers, key.number)
		}
	}
	slices.Sort(numbers)
	return numbers
}

func latestStatesOfOurs(facts []contract.Fact) map[pullKey]contract.PrStateChanged {
	ours := map[pullKey]bool{}
	states := map[pullKey]contract.PrStateChanged{}
	for _, f := range facts {
		switch f := f.(type) {
		case contract.PrCreated:
			ours[pullKey{f.Repository, f.Number}] = true
		case contract.PrStateChanged:
			states[pullKey{f.Repository, f.Number}] = f
		}
	}
	for key := range states {
		if !ours[key] {
			delete(states, key)
		}
	}
	return states
}
