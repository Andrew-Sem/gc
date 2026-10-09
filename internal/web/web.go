package web

import (
	"bytes"
	"embed"
	"net/http"
	"net/url"
	"strings"

	"github.com/Andrew-Sem/gc/internal/domain"
	"github.com/Andrew-Sem/gc/specs/contract"
)

//go:embed static
var static embed.FS

const htmxConfig = `{"responseHandling":[
	{"code":"204","swap":false},
	{"code":"[23]..","swap":true},
	{"code":"422","swap":true},
	{"code":"502","swap":true,"error":true},
	{"code":"[45]..","swap":false,"error":true}
]}`

type problem int

const (
	noProblem problem = iota
	noAccess
	malformedName
	githubSilent
)

type server struct {
	journal contract.Journal
	github  contract.GitHub
}

func New(deps contract.Dependencies) http.Handler {
	s := &server{journal: deps.Journal, github: deps.GitHub}
	mux := http.NewServeMux()
	mux.Handle("GET /static/", http.FileServerFS(static))
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
	fullName := strings.ToLower(strings.Trim(r.FormValue("repo"), " "))
	if !domain.ValidFullName(fullName) {
		s.renderPanel(w, r, http.StatusUnprocessableEntity, malformedName)
		return
	}
	if domain.IsConnected(s.journal.Facts(), fullName) {
		s.renderPanel(w, r, http.StatusOK, noProblem)
		return
	}
	ok, err := s.github.CanAccess(fullName)
	if err != nil {
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
	fullName := r.PathValue("owner") + "/" + r.PathValue("name")
	if domain.IsConnected(s.journal.Facts(), fullName) {
		s.journal.Append(contract.RepositoryDisconnected{FullName: fullName})
	}
	s.renderPanel(w, r, http.StatusOK, noProblem)
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
