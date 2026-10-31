package web

import (
	"bytes"
	"errors"
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
	mux.HandleFunc("POST /repositories", s.connectRepository)
	mux.HandleFunc("POST /repositories/{owner}/{name}/disconnect", s.disconnectRepository)
	mux.HandleFunc("GET /runs", s.runsPage)
	mux.HandleFunc("POST /webhooks/github", s.receiveWebhook)
	return mux
}

func (s *server) repositoriesPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	repositories := domain.ConnectedRepositories(s.journal.Facts())
	if err := repositoriesView(repositories).Render(r.Context(), w); err != nil {
		http.Error(w, "cannot render the page", http.StatusInternalServerError)
	}
}

func (s *server) runsPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := runsView(domain.Runs(s.journal.Facts()), time.Now()).Render(r.Context(), w); err != nil {
		http.Error(w, "cannot render the page", http.StatusInternalServerError)
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

func normalize(fullName string) string {
	return strings.ToLower(strings.TrimSpace(fullName))
}

func (s *server) renderPanel(w http.ResponseWriter, r *http.Request, status int, p problem) {
	repositories := domain.ConnectedRepositories(s.journal.Facts())
	var panel bytes.Buffer
	if err := repositoriesPanel(repositories, p).Render(r.Context(), &panel); err != nil {
		http.Error(w, "cannot render the panel", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	panel.WriteTo(w)
}

func disconnectPath(fullName string) string {
	owner, name, _ := strings.Cut(fullName, "/")
	return "/repositories/" + url.PathEscape(owner) + "/" + url.PathEscape(name) + "/disconnect"
}
