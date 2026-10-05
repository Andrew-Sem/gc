package web

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/Andrew-Sem/gc/internal/domain"
	"github.com/Andrew-Sem/gc/internal/journal"
)

type GitHub interface {
	CanAccess(fullName string) (bool, error)
}

type server struct {
	journal *journal.Journal
	github  GitHub
}

func New(j *journal.Journal, gh GitHub) http.Handler {
	s := &server{journal: j, github: gh}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /repositories", s.repositoriesPage)
	mux.HandleFunc("POST /repositories", s.connectRepository)
	mux.HandleFunc("POST /repositories/{owner}/{name}/disconnect", s.disconnectRepository)
	return mux
}

func (s *server) repositoriesPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	repositories := domain.ConnectedRepositories(s.journal.Facts())
	if err := repositoriesView(repositories).Render(r.Context(), w); err != nil {
		http.Error(w, "cannot render the page", http.StatusInternalServerError)
	}
}

func (s *server) connectRepository(w http.ResponseWriter, r *http.Request) {
	fullName := r.FormValue("repo")
	if !domain.ValidFullName(fullName) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}
	if domain.IsConnected(s.journal.Facts(), fullName) {
		return
	}
	ok, err := s.github.CanAccess(fullName)
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	if !ok {
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}
	s.journal.Append(domain.RepositoryConnected{FullName: fullName})
}

func (s *server) disconnectRepository(w http.ResponseWriter, r *http.Request) {
	fullName := r.PathValue("owner") + "/" + r.PathValue("name")
	if domain.IsConnected(s.journal.Facts(), fullName) {
		s.journal.Append(domain.RepositoryDisconnected{FullName: fullName})
	}
}

func disconnectPath(fullName string) string {
	owner, name, _ := strings.Cut(fullName, "/")
	return "/repositories/" + url.PathEscape(owner) + "/" + url.PathEscape(name) + "/disconnect"
}
