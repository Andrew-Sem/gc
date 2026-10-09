package domain

import (
	"strings"

	"github.com/Andrew-Sem/gc/specs/contract"
)

func IsConnected(facts []contract.Fact, fullName string) bool {
	connected := false
	for _, f := range facts {
		switch f := f.(type) {
		case contract.RepositoryConnected:
			if f.FullName == fullName {
				connected = true
			}
		case contract.RepositoryDisconnected:
			if f.FullName == fullName {
				connected = false
			}
		}
	}
	return connected
}

func ValidFullName(fullName string) bool {
	owner, name, found := strings.Cut(fullName, "/")
	return found && owner != "" && name != "" && !strings.Contains(name, "/") && !strings.Contains(fullName, " ")
}

func ConnectedRepositories(facts []contract.Fact) []string {
	seen := map[string]bool{}
	var repositories []string
	for i := len(facts) - 1; i >= 0; i-- {
		switch f := facts[i].(type) {
		case contract.RepositoryConnected:
			if !seen[f.FullName] {
				repositories = append(repositories, f.FullName)
			}
			seen[f.FullName] = true
		case contract.RepositoryDisconnected:
			seen[f.FullName] = true
		}
	}
	return repositories
}
