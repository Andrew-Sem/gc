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

func HasRoom(facts []contract.Fact, repository string) (open, limit int, room bool) {
	open, limit = openPRs(facts, repository), RepositorySettings(facts, repository).Limit
	return open, limit, open < limit
}

func Admit(journal contract.Journal, repository string) bool {
	if open, limit, room := HasRoom(journal.Facts(), repository); !room {
		journal.Append(contract.AdmissionDeclined{Repository: repository, Open: open, Limit: limit})
		return false
	}
	journal.Append(contract.AdmissionApproved{Repository: repository})
	return true
}

func openPRs(facts []contract.Fact, repository string) int {
	return len(OpenPRs(facts, repository))
}

func OpenPRs(facts []contract.Fact, repository string) []int {
	var order []int
	open := map[int]bool{}
	for _, f := range facts {
		switch f := f.(type) {
		case contract.PrCreated:
			if f.Repository == repository {
				if _, known := open[f.Number]; !known {
					order = append(order, f.Number)
				}
				open[f.Number] = true
			}
		case contract.PrStateChanged:
			if _, ours := open[f.Number]; ours && f.Repository == repository {
				open[f.Number] = f.Open
			}
		}
	}
	var numbers []int
	for _, number := range order {
		if open[number] {
			numbers = append(numbers, number)
		}
	}
	return numbers
}
