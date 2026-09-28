// Package app — команды оператора поверх журнала фактов и портов наружу.
package app

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"

	"github.com/andrew-semyonov/garbage-collector/internal/domain"
)

// Journal — журнал фактов, единственный источник правды. Только дописывается.
type Journal interface {
	Append(ctx context.Context, facts ...domain.Fact) error
	Facts(ctx context.Context) ([]domain.Fact, error)
}

// GitHub — то, что приложению нужно от GitHub.
type GitHub interface {
	// RepositoryAccessible отвечает, существует ли репозиторий и видит ли его токен.
	RepositoryAccessible(ctx context.Context, repo string) (bool, error)
}

// Deps — всё, что приложение получает снаружи. В спеках это журнал в памяти и клиент
// поддельного GitHub API.
type Deps struct {
	Journal Journal
	GitHub  GitHub
	Log     *slog.Logger
}

var (
	// ErrNoAccess — репозитория нет или токен его не видит.
	ErrNoAccess = errors.New("нет доступа к репозиторию")
	// ErrGitHubUnavailable — GitHub не ответил, доступ проверить не удалось.
	ErrGitHubUnavailable = errors.New("не удалось проверить доступ в GitHub")
)

// App выполняет команды оператора.
type App struct {
	journal Journal
	github  GitHub
	log     *slog.Logger

	// mu выполняет команды по одной: прочитать журнал → решить → дописать. Без него два
	// одновременных «Подключить» записали бы два факта. Процесс один, этого достаточно.
	mu sync.Mutex
}

func New(deps Deps) *App {
	log := deps.Log
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &App{journal: deps.Journal, github: deps.GitHub, log: log}
}

// Repositories — подключённые репозитории в порядке подключения.
func (a *App) Repositories(ctx context.Context) ([]string, error) {
	state, err := a.state(ctx)
	if err != nil {
		return nil, err
	}
	return state.Connected(), nil
}

// Connect подключает репозиторий по строке owner/name из формы.
func (a *App) Connect(ctx context.Context, input string) error {
	repo, err := domain.ParseRepo(input)
	if err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	state, err := a.state(ctx)
	if err != nil {
		return err
	}
	if state.IsConnected(repo) {
		return nil
	}
	accessible, err := a.github.RepositoryAccessible(ctx, repo)
	if err != nil {
		a.log.Error("проверка доступа в GitHub не удалась", "repo", repo, "err", err)
		return ErrGitHubUnavailable
	}
	if !accessible {
		return ErrNoAccess
	}
	return a.journal.Append(ctx, state.Connect(repo)...)
}

// Disconnect отключает репозиторий. Отключение неподключённого ничего не пишет.
func (a *App) Disconnect(ctx context.Context, repo string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	state, err := a.state(ctx)
	if err != nil {
		return err
	}
	return a.journal.Append(ctx, state.Disconnect(strings.TrimSpace(repo))...)
}

func (a *App) state(ctx context.Context) (domain.Repositories, error) {
	facts, err := a.journal.Facts(ctx)
	if err != nil {
		return domain.Repositories{}, err
	}
	return domain.FoldRepositories(facts), nil
}
