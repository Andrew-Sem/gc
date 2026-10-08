package domain

import (
	"maps"
	"slices"
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
	return found && owner != "" && name != "" && !strings.Contains(name, "/")
}

func ConnectedRepositories(facts []contract.Fact) []string {
	connected := map[string]bool{}
	for _, f := range facts {
		switch f := f.(type) {
		case contract.RepositoryConnected:
			connected[f.FullName] = true
		case contract.RepositoryDisconnected:
			delete(connected, f.FullName)
		}
	}
	return slices.Sorted(maps.Keys(connected))
}
