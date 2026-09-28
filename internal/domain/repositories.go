package domain

import (
	"errors"
	"regexp"
	"slices"
	"strings"
)

// ErrInvalidRepo — строка не похожа на owner/name.
var ErrInvalidRepo = errors.New("нужно имя вида owner/name")

var repoName = regexp.MustCompile(`^[A-Za-z0-9-]+/[A-Za-z0-9._-]+$`)

// ParseRepo проверяет строку из формы и возвращает имя репозитория без пробелов по краям.
func ParseRepo(input string) (string, error) {
	repo := strings.TrimSpace(input)
	_, name, _ := strings.Cut(repo, "/")
	if !repoName.MatchString(repo) || name == "." || name == ".." {
		return "", ErrInvalidRepo
	}
	return repo, nil
}

// Repositories — подключённые репозитории, свёрнутые из журнала.
type Repositories struct {
	connected []string // в порядке подключения
}

// FoldRepositories сворачивает журнал в состояние подключений.
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

// Connected — подключённые репозитории в порядке подключения.
func (r Repositories) Connected() []string {
	return slices.Clone(r.connected)
}

func (r Repositories) IsConnected(repo string) bool {
	return slices.Contains(r.connected, repo)
}

// Connect решает, что записать, когда доступ к репозиторию уже проверен.
// Уже подключённый репозиторий ничего не порождает.
func (r Repositories) Connect(repo string) []Fact {
	if r.IsConnected(repo) {
		return nil
	}
	return []Fact{RepositoryConnected{Repo: repo}}
}

// Disconnect решает, что записать при отключении. Неподключённый репозиторий ничего не
// порождает.
func (r Repositories) Disconnect(repo string) []Fact {
	if !r.IsConnected(repo) {
		return nil
	}
	return []Fact{RepositoryDisconnected{Repo: repo}}
}
