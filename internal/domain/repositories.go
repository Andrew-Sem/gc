package domain

import (
	"errors"
	"regexp"
	"slices"
	"strings"
)

var ErrInvalidRepo = errors.New("нужно имя вида owner/name")

var repoName = regexp.MustCompile(`^[A-Za-z0-9-]+/[A-Za-z0-9._-]+$`)

func ParseRepo(input string) (string, error) {
	repo := strings.TrimSpace(input)
	_, name, _ := strings.Cut(repo, "/")
	if !repoName.MatchString(repo) || name == "." || name == ".." {
		return "", ErrInvalidRepo
	}
	return repo, nil
}

type Repositories struct {
	connected []string
}

func FoldRepositories(facts []Fact) Repositories {
	var r Repositories
	for _, fact := range facts {
		switch f := fact.(type) {
		case RepositoryConnected:
			if !r.IsConnected(f.Repo) {
				r.connected = append(r.connected, f.Repo)
			}
		case RepositoryDisconnected:
			r.connected = slices.DeleteFunc(r.connected, func(repo string) bool { return repo == f.Repo })
		}
	}
	return r
}

func (r Repositories) Connected() []string {
	return slices.Clone(r.connected)
}

func (r Repositories) IsConnected(repo string) bool {
	return slices.Contains(r.connected, repo)
}

func (r Repositories) Connect(repo string) []Fact {
	if r.IsConnected(repo) {
		return nil
	}
	return []Fact{RepositoryConnected{Repo: repo}}
}

func (r Repositories) Disconnect(repo string) []Fact {
	if !r.IsConnected(repo) {
		return nil
	}
	return []Fact{RepositoryDisconnected{Repo: repo}}
}
