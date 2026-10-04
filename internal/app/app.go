package app

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"

	"github.com/Andrew-Sem/gc/internal/domain"
)

type Journal interface {
	Append(ctx context.Context, facts ...domain.Fact) error
	Facts(ctx context.Context) ([]domain.Fact, error)
}

type GitHub interface {
	RepositoryAccessible(ctx context.Context, repo string) (bool, error)
}

type Deps struct {
	Journal Journal
	GitHub  GitHub
	Log     *slog.Logger
}

var (
	ErrNoAccess          = errors.New("нет доступа к репозиторию")
	ErrGitHubUnavailable = errors.New("не удалось проверить доступ в GitHub")
)

type App struct {
	journal Journal
	github  GitHub
	log     *slog.Logger

	mu sync.Mutex
}

func New(deps Deps) *App {
	log := deps.Log
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &App{journal: deps.Journal, github: deps.GitHub, log: log}
}

func (a *App) Repositories(ctx context.Context) ([]string, error) {
	state, err := a.state(ctx)
	if err != nil {
		return nil, err
	}
	return state.Connected(), nil
}

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
