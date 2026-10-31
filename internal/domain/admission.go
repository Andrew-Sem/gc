package domain

import "github.com/Andrew-Sem/gc/specs/contract"

func WakeUp(journal contract.Journal) {
	for _, repository := range ConnectedRepositories(journal.Facts()) {
		journal.Append(contract.AdmissionApproved{Repository: repository})
	}
}
