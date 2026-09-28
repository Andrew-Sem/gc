// Package web — HTTP-админка: страницы, формы и фрагменты htmx.
package web

import (
	"bytes"
	"context"
	"embed"
	"errors"
	"html/template"
	"net/http"

	"github.com/andrew-semyonov/garbage-collector/internal/app"
	"github.com/andrew-semyonov/garbage-collector/internal/domain"
)

//go:embed templates/*.html
var templateFiles embed.FS

var templates = template.Must(template.ParseFS(templateFiles, "templates/*.html"))

// New собирает HTTP-обработчик админки.
func New(a *app.App) http.Handler {
	h := &handler{app: a}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /repositories", h.page)
	mux.HandleFunc("POST /repositories", h.connect)
	mux.HandleFunc("POST /repositories/disconnect", h.disconnect)
	return mux
}

type handler struct {
	app *app.App
}

// section — данные блока со списком: его отдают и страница, и ответы на формы.
type section struct {
	Repositories []string
	Error        string // код ошибки для data-error
	Message      string // та же ошибка словами
	Input        string // что ввёл оператор, чтобы при ошибке не набирать заново
}

func (h *handler) page(w http.ResponseWriter, r *http.Request) {
	s, err := h.section(r.Context())
	if err != nil {
		internalError(w)
		return
	}
	render(w, http.StatusOK, "page", s)
}

func (h *handler) connect(w http.ResponseWriter, r *http.Request) {
	input := r.FormValue("repo")
	status, code, message := connectOutcome(h.app.Connect(r.Context(), input))
	if status == http.StatusInternalServerError {
		internalError(w)
		return
	}
	s, err := h.section(r.Context())
	if err != nil {
		internalError(w)
		return
	}
	if code != "" {
		s.Error, s.Message, s.Input = code, message, input
	}
	render(w, status, "section", s)
}

func (h *handler) disconnect(w http.ResponseWriter, r *http.Request) {
	if err := h.app.Disconnect(r.Context(), r.FormValue("repo")); err != nil {
		internalError(w)
		return
	}
	s, err := h.section(r.Context())
	if err != nil {
		internalError(w)
		return
	}
	render(w, http.StatusOK, "section", s)
}

func (h *handler) section(ctx context.Context) (section, error) {
	repos, err := h.app.Repositories(ctx)
	return section{Repositories: repos}, err
}

// connectOutcome переводит результат подключения в статус ответа и сообщение на странице.
func connectOutcome(err error) (status int, code, message string) {
	switch {
	case err == nil:
		return http.StatusOK, "", ""
	case errors.Is(err, domain.ErrInvalidRepo):
		return http.StatusUnprocessableEntity, "invalid-name",
			"Нужно имя вида owner/name, например acme/garbage-collector."
	case errors.Is(err, app.ErrNoAccess):
		return http.StatusUnprocessableEntity, "no-access",
			"Нет доступа: такого репозитория нет или токен его не видит."
	case errors.Is(err, app.ErrGitHubUnavailable):
		return http.StatusBadGateway, "github-unavailable",
			"GitHub не ответил, проверить доступ не удалось. Попробуйте ещё раз."
	default:
		return http.StatusInternalServerError, "", ""
	}
}

// render сначала собирает страницу целиком, чтобы ошибка шаблона не оставила полуответ.
func render(w http.ResponseWriter, status int, name string, data any) {
	var buf bytes.Buffer
	if err := templates.ExecuteTemplate(&buf, name, data); err != nil {
		internalError(w)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = buf.WriteTo(w)
}

func internalError(w http.ResponseWriter) {
	http.Error(w, "Внутренняя ошибка", http.StatusInternalServerError)
}
