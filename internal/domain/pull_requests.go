package domain

import "github.com/Andrew-Sem/gc/specs/contract"

func IsOurPR(facts []contract.Fact, repository string, number int) bool {
	if !IsConnected(facts, repository) {
		return false
	}
	for _, f := range facts {
		if created, ok := f.(contract.PrCreated); ok && created.Repository == repository && created.Number == number {
			return true
		}
	}
	return false
}

func IsNewSnapshot(facts []contract.Fact, snapshot contract.PrStateChanged) bool {
	for i := len(facts) - 1; i >= 0; i-- {
		known, ok := facts[i].(contract.PrStateChanged)
		if !ok || known.Repository != snapshot.Repository || known.Number != snapshot.Number {
			continue
		}
		if snapshot.UpdatedAt.Before(known.UpdatedAt) {
			return false
		}
		known.UpdatedAt = snapshot.UpdatedAt
		return known != snapshot
	}
	return true
}

func IsNewCheckRunState(facts []contract.Fact, repository string, number int, run contract.CheckRun) bool {
	for i := len(facts) - 1; i >= 0; i-- {
		known, ok := facts[i].(contract.CheckRunStateChanged)
		if !ok || known.Repository != repository || known.Number != number || known.CheckRun.ID != run.ID {
			continue
		}
		if known.CheckRun.Status == "completed" && run.Status != "completed" {
			return false
		}
		return known.CheckRun != run
	}
	return true
}
