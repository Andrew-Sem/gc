package domain

import (
	"time"

	"github.com/Andrew-Sem/gc/specs/contract"
)

func WakeUp(journal contract.Journal, now time.Time, defaultBaseURL string) []contract.AdmissionApproved {
	var approved []contract.AdmissionApproved
	for _, repository := range ConnectedRepositories(journal.Facts()) {
		if !RepositorySettings(journal.Facts(), repository).Due(now) {
			continue
		}
		if approval, ok := Admit(journal, repository, defaultBaseURL); ok {
			approved = append(approved, approval)
		}
	}
	return approved
}

func HasRoom(facts []contract.Fact, repository string) (open, limit int, room bool) {
	open, limit = openPRs(facts, repository), RepositorySettings(facts, repository).Limit
	return open, limit, open < limit
}

func Admit(journal contract.Journal, repository, defaultBaseURL string) (contract.AdmissionApproved, bool) {
	facts := journal.Facts()
	if open, limit, room := HasRoom(facts, repository); !room {
		journal.Append(contract.AdmissionDeclined{Repository: repository, Open: open, Limit: limit})
		return contract.AdmissionApproved{}, false
	}
	approval := contract.AdmissionApproved{Repository: repository, OpenAIBaseURL: OpenAIBaseURL(facts, defaultBaseURL)}
	journal.Append(approval)
	return approval, true
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
