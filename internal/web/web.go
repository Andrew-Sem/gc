package web

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/Andrew-Sem/gc/internal/domain"
	"github.com/Andrew-Sem/gc/internal/github"
	"github.com/Andrew-Sem/gc/specs/contract"
	"github.com/Andrew-Sem/gc/ui"
)

type problem int

const (
	noProblem problem = iota
	noAccess
	malformedName
	githubSilent
	tokenRejected
	githubFailed
	rateLimited
	moved
	runRequested
	notConnected
	noRoom
	sweepRequested
)

type server struct {
	journal       contract.Journal
	github        *github.Client
	webhookSecret string
	webhooks      sync.Mutex
	deliveries    map[string]bool
}

func New(deps contract.Dependencies) http.Handler {
	s := &server{journal: deps.Journal, github: github.New(deps.GitHub), webhookSecret: deps.WebhookSecret, deliveries: map[string]bool{}}
	mux := http.NewServeMux()
	mux.Handle("GET /static/", ui.Static())
	mux.HandleFunc("GET /repositories", s.repositoriesPage)
	sameSite := http.NewCrossOriginProtection()
	mux.Handle("POST /repositories", sameSite.Handler(http.HandlerFunc(s.connectRepository)))
	mux.Handle("POST /repositories/{owner}/{name}/disconnect", sameSite.Handler(http.HandlerFunc(s.disconnectRepository)))
	mux.Handle("POST /repositories/{owner}/{name}/run", sameSite.Handler(http.HandlerFunc(s.requestRun)))
	mux.Handle("POST /repositories/{owner}/{name}/sweep", sameSite.Handler(http.HandlerFunc(s.requestSweep)))
	mux.HandleFunc("GET /repositories/{owner}/{name}/settings", s.settingsPage)
	mux.Handle("POST /repositories/{owner}/{name}/settings", sameSite.Handler(http.HandlerFunc(s.saveSettings)))
	mux.HandleFunc("GET /runs", s.runsPage)
	mux.HandleFunc("GET /pulls", s.pullsPage)
	mux.HandleFunc("GET /runs/panel", s.runsPanel)
	mux.HandleFunc("GET /runs/events", s.runsEvents)
	mux.HandleFunc("POST /webhooks/github", s.receiveWebhook)
	return mux
}

func (s *server) repositoriesPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := repositoriesView(s.repositoryRows()).Render(r.Context(), w); err != nil {
		http.Error(w, "cannot render the page", http.StatusInternalServerError)
	}
}

func (s *server) runsPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := runsView(domain.Runs(s.journal.Facts()), time.Now()).Render(r.Context(), w); err != nil {
		http.Error(w, "cannot render the page", http.StatusInternalServerError)
	}
}

func (s *server) pullsPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := pullsView(domain.OpenPulls(s.journal.Facts())).Render(r.Context(), w); err != nil {
		http.Error(w, "cannot render the page", http.StatusInternalServerError)
	}
}

func (s *server) runsPanel(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := runsPanel(domain.Runs(s.journal.Facts()), time.Now()).Render(r.Context(), w); err != nil {
		http.Error(w, "cannot render the panel", http.StatusInternalServerError)
	}
}

func (s *server) runsEvents(w http.ResponseWriter, r *http.Request) {
	changes, cancel := s.journal.Subscribe()
	defer cancel()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	stream := http.NewResponseController(w)
	if err := stream.Flush(); err != nil {
		return
	}
	for {
		select {
		case <-r.Context().Done():
			return
		case <-changes:
			if _, err := io.WriteString(w, "data: changed\n\n"); err != nil {
				return
			}
			if err := stream.Flush(); err != nil {
				return
			}
		}
	}
}

func (s *server) connectRepository(w http.ResponseWriter, r *http.Request) {
	fullName := normalize(r.FormValue("repo"))
	if !domain.ValidFullName(fullName) {
		s.renderPanel(w, r, http.StatusUnprocessableEntity, malformedName)
		return
	}
	if domain.IsConnected(s.journal.Facts(), fullName) {
		s.renderPanel(w, r, http.StatusOK, noProblem)
		return
	}
	ok, err := s.github.CanAccess(r.Context(), fullName)
	switch {
	case errors.Is(err, github.ErrTokenRejected):
		s.renderPanel(w, r, http.StatusBadGateway, tokenRejected)
		return
	case errors.Is(err, github.ErrRateLimited):
		s.renderPanel(w, r, http.StatusBadGateway, rateLimited)
		return
	case errors.Is(err, github.ErrMoved):
		s.renderPanel(w, r, http.StatusUnprocessableEntity, moved)
		return
	case errors.Is(err, github.ErrFailed):
		s.renderPanel(w, r, http.StatusBadGateway, githubFailed)
		return
	case err != nil:
		s.renderPanel(w, r, http.StatusBadGateway, githubSilent)
		return
	}
	if !ok {
		s.renderPanel(w, r, http.StatusUnprocessableEntity, noAccess)
		return
	}
	s.journal.Append(contract.RepositoryConnected{FullName: fullName})
	s.renderPanel(w, r, http.StatusOK, noProblem)
}

func (s *server) disconnectRepository(w http.ResponseWriter, r *http.Request) {
	fullName := normalize(r.PathValue("owner") + "/" + r.PathValue("name"))
	if domain.IsConnected(s.journal.Facts(), fullName) {
		s.journal.Append(contract.RepositoryDisconnected{FullName: fullName})
	}
	s.renderPanel(w, r, http.StatusOK, noProblem)
}

func (s *server) requestRun(w http.ResponseWriter, r *http.Request) {
	fullName := normalize(r.PathValue("owner") + "/" + r.PathValue("name"))
	if !domain.IsConnected(s.journal.Facts(), fullName) {
		s.renderPanel(w, r, http.StatusUnprocessableEntity, notConnected)
		return
	}
	open, limit, room := domain.HasRoom(s.journal.Facts(), fullName)
	s.journal.Append(contract.RunRequested{Repository: fullName})
	if !room {
		s.renderPanelWith(w, r, http.StatusOK, panelState{problem: noRoom, repository: fullName, open: open, limit: limit})
		return
	}
	s.renderPanel(w, r, http.StatusOK, runRequested)
}

func (s *server) requestSweep(w http.ResponseWriter, r *http.Request) {
	fullName := normalize(r.PathValue("owner") + "/" + r.PathValue("name"))
	if !domain.IsConnected(s.journal.Facts(), fullName) {
		s.renderPanel(w, r, http.StatusUnprocessableEntity, notConnected)
		return
	}
	s.journal.Append(contract.SweepRequested{Repository: fullName})
	s.renderPanel(w, r, http.StatusOK, sweepRequested)
}

func normalize(fullName string) string {
	return strings.ToLower(strings.TrimSpace(fullName))
}

type panelState struct {
	problem     problem
	repository  string
	open, limit int
}

func (s *server) renderPanel(w http.ResponseWriter, r *http.Request, status int, p problem) {
	s.renderPanelWith(w, r, status, panelState{problem: p})
}

func (s *server) renderPanelWith(w http.ResponseWriter, r *http.Request, status int, state panelState) {
	var panel bytes.Buffer
	if err := repositoriesPanel(s.repositoryRows(), state).Render(r.Context(), &panel); err != nil {
		http.Error(w, "cannot render the panel", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	panel.WriteTo(w)
}

type repositoryRow struct {
	Name     string
	Settings domain.Settings
}

func (s *server) repositoryRows() []repositoryRow {
	facts := s.journal.Facts()
	var rows []repositoryRow
	for _, name := range domain.ConnectedRepositories(facts) {
		rows = append(rows, repositoryRow{Name: name, Settings: domain.RepositorySettings(facts, name)})
	}
	return rows
}

func disconnectPath(fullName string) string {
	return repositoryPath(fullName) + "/disconnect"
}

func settingsPath(fullName string) string {
	return repositoryPath(fullName) + "/settings"
}

func sweepPath(fullName string) string {
	return repositoryPath(fullName) + "/sweep"
}

func runPath(fullName string) string {
	return repositoryPath(fullName) + "/run"
}

func repositoryPath(fullName string) string {
	owner, name, _ := strings.Cut(fullName, "/")
	return "/repositories/" + url.PathEscape(owner) + "/" + url.PathEscape(name)
}
