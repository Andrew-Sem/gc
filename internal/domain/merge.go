package domain

import "github.com/Andrew-Sem/gc/specs/contract"

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
