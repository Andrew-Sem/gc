package domain

import (
	"regexp"
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

var fullNamePattern = regexp.MustCompile(`^[a-z0-9-]+/[a-z0-9._-]+$`)

func ValidFullName(fullName string) bool {
	return fullNamePattern.MatchString(fullName) && !strings.HasSuffix(fullName, "/.") && !strings.HasSuffix(fullName, "/..")
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
