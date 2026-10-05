package web

import (
	"fmt"
	"net/http"

	"github.com/Andrew-Sem/gc/internal/domain"
	"github.com/Andrew-Sem/gc/internal/journal"
)

type GitHub interface {
	CanAccess(fullName string) (bool, error)
}

func New(j *journal.Journal, gh GitHub) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /repositories", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><title>Репозитории</title><h1>Репозитории</h1>`)
	})
	mux.HandleFunc("POST /repositories/{owner}/{name}/disconnect", func(w http.ResponseWriter, r *http.Request) {
		fullName := r.PathValue("owner") + "/" + r.PathValue("name")
		if domain.IsConnected(j.Facts(), fullName) {
			j.Append(domain.RepositoryDisconnected{FullName: fullName})
		}
	})
	mux.HandleFunc("POST /repositories", func(w http.ResponseWriter, r *http.Request) {
		fullName := r.FormValue("repo")
		ok, _ := gh.CanAccess(fullName)
		if !ok {
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		}
		j.Append(domain.RepositoryConnected{FullName: fullName})
	})
	return mux
}
