package domain

import (
	"time"

	"github.com/Andrew-Sem/gc/specs/contract"
)

func WakeUp(journal contract.Journal, now time.Time) []string {
	var approved []string
	for _, repository := range ConnectedRepositories(journal.Facts()) {
		if !RepositorySettings(journal.Facts(), repository).Due(now) {
			continue
		}
		if Admit(journal, repository) {
			approved = append(approved, repository)
		}
	}
	return approved
}

func Admit(journal contract.Journal, repository string) bool {
	limit := RepositorySettings(journal.Facts(), repository).Limit
	if open := openPRs(journal.Facts(), repository); open >= limit {
		journal.Append(contract.AdmissionDeclined{Repository: repository, Open: open, Limit: limit})
		return false
	}
	journal.Append(contract.AdmissionApproved{Repository: repository})
	return true
}

func openPRs(facts []contract.Fact, repository string) int {
	open := map[int]bool{}
	for _, f := range facts {
		switch f := f.(type) {
		case contract.PrCreated:
			if f.Repository == repository {
				open[f.Number] = true
			}
		case contract.PrStateChanged:
			if _, ours := open[f.Number]; ours && f.Repository == repository {
				open[f.Number] = f.Open
			}
		}
	}
	count := 0
	for _, isOpen := range open {
		if isOpen {
			count++
		}
	}
	return count
}
