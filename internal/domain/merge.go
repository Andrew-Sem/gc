package domain

import (
	"slices"
	"strings"

	"github.com/Andrew-Sem/gc/specs/contract"
)

type MergeState struct {
	Draft          bool
	RequiredChecks int
	Mergeable      contract.Mergeable
	MergeableState string
}

func MergeVerdict(state MergeState) (reason string, merge bool) {
	switch {
	case state.Draft:
		return "draft", false
	case state.RequiredChecks == 0:
		return "no required checks", false
	case state.Mergeable == contract.MergeableUnknown:
		return "GitHub is still checking", false
	case state.Mergeable == contract.MergeableNo:
		return "conflicts", false
	case state.MergeableState == "blocked":
		return "required checks have not passed", false
	case state.MergeableState == "behind":
		return "behind the base branch", false
	case state.MergeableState == "clean", state.MergeableState == "unstable", state.MergeableState == "has_hooks":
		return "", true
	}
	return "not mergeable: " + state.MergeableState, false
}

type OpenPull struct {
	Repository string
	Number     int
	Checks     string
	Decision   string
	Reason     string
}

func OpenPulls(facts []contract.Fact) []OpenPull {
	type key struct {
		repository string
		number     int
	}
	decisions := map[key]contract.Fact{}
	var order []key
	for _, f := range facts {
		switch f := f.(type) {
		case contract.PrCreated:
			order = append(order, key{f.Repository, f.Number})
		case contract.MergeApproved:
			decisions[key{f.Repository, f.Number}] = f
		case contract.MergeDeclined:
			decisions[key{f.Repository, f.Number}] = f
		case contract.MergeFailed:
			decisions[key{f.Repository, f.Number}] = f
		}
	}
	var pulls []OpenPull
	seen := map[key]bool{}
	for i := len(order) - 1; i >= 0; i-- {
		k := order[i]
		if seen[k] || !IsConnected(facts, k.repository) || !slices.Contains(OpenPRs(facts, k.repository), k.number) {
			continue
		}
		seen[k] = true
		pull := OpenPull{Repository: k.repository, Number: k.number, Checks: ChecksVerdict(facts, k.repository, k.number), Decision: "no sweep yet"}
		switch decision := decisions[k].(type) {
		case contract.MergeApproved:
			pull.Decision = "merging"
		case contract.MergeDeclined:
			pull.Decision, pull.Reason = "not merged", decision.Reason
		case contract.MergeFailed:
			pull.Decision, pull.Reason = "merge failed", decision.Reason
		}
		if !RepositorySettings(facts, k.repository).Merge {
			pull.Decision, pull.Reason = "merging is off", ""
		}
		pulls = append(pulls, pull)
	}
	return pulls
}

var passingConclusions = []string{"success", "neutral", "skipped"}

func ChecksVerdict(facts []contract.Fact, repository string, number int) string {
	head := ""
	var observed *contract.RequiredChecksObserved
	attempts := map[int64]contract.CheckRun{}
	for _, f := range facts {
		switch f := f.(type) {
		case contract.PrStateChanged:
			if f.Repository == repository && f.Number == number {
				head = f.HeadSHA
			}
		case contract.RequiredChecksObserved:
			if f.Repository == repository && f.Number == number {
				observed = &f
			}
		case contract.CheckRunStateChanged:
			if f.Repository == repository && f.Number == number {
				attempts[f.CheckRun.ID] = f.CheckRun
			}
		}
	}
	if head == "" || observed == nil || observed.HeadSHA != head {
		return "unknown"
	}
	if len(observed.Required) == 0 {
		return "no required checks"
	}
	var failed, pending []string
	for _, required := range observed.Required {
		var counted *contract.CheckRun
		for _, run := range attempts {
			if run.HeadSHA != head || run.Name != required.Name || (required.AppID != 0 && run.AppID != required.AppID) {
				continue
			}
			if counted == nil || run.StartedAt.After(counted.StartedAt) || (run.StartedAt.Equal(counted.StartedAt) && run.ID > counted.ID) {
				counted = &run
			}
		}
		switch {
		case counted == nil || counted.Status != "completed":
			pending = append(pending, required.Name)
		case !slices.Contains(passingConclusions, counted.Conclusion):
			failed = append(failed, required.Name)
		}
	}
	switch {
	case len(failed) > 0:
		return "failed: " + strings.Join(failed, ", ")
	case len(pending) > 0:
		return "pending: " + strings.Join(pending, ", ")
	}
	return "passed"
}
