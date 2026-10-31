package domain

import "github.com/Andrew-Sem/gc/specs/contract"

const openPRLimit = 12

func WakeUp(journal contract.Journal) {
	facts := journal.Facts()
	for _, repository := range ConnectedRepositories(facts) {
		if open := openPRs(facts, repository); open < openPRLimit {
			journal.Append(contract.AdmissionApproved{Repository: repository})
		} else {
			journal.Append(contract.AdmissionDeclined{Repository: repository, Open: open, Limit: openPRLimit})
		}
	}
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
