package web

import (
	"bytes"
	"net/http"
	"time"

	"github.com/Andrew-Sem/gc/internal/domain"
	"github.com/Andrew-Sem/gc/specs/contract"
)

type instanceForm struct {
	BaseURL string
	Saved   bool
	Problem string
}

func (s *server) instancePage(w http.ResponseWriter, r *http.Request) {
	form := instanceForm{BaseURL: domain.OpenAIBaseURL(s.journal.Facts(), s.defaultBaseURL)}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := instanceView(form).Render(r.Context(), w); err != nil {
		http.Error(w, "cannot render the page", http.StatusInternalServerError)
	}
}

func (s *server) saveInstanceSettings(w http.ResponseWriter, r *http.Request) {
	baseURL, valid := domain.NormalizeBaseURL(r.FormValue("openai_base_url"))
	if !valid {
		s.renderInstance(w, r, http.StatusUnprocessableEntity, instanceForm{BaseURL: r.FormValue("openai_base_url"), Problem: "Enter an absolute http or https URL without credentials, query, or fragment"})
		return
	}
	if baseURL != domain.OpenAIBaseURL(s.journal.Facts(), s.defaultBaseURL) {
		s.journal.Append(contract.SettingsChanged{OpenAIBaseURL: baseURL, At: time.Now().UTC()})
	}
	s.renderInstance(w, r, http.StatusOK, instanceForm{BaseURL: baseURL, Saved: true})
}

func (s *server) renderInstance(w http.ResponseWriter, r *http.Request, status int, form instanceForm) {
	var panel bytes.Buffer
	if err := instancePanel(form).Render(r.Context(), &panel); err != nil {
		http.Error(w, "cannot render the panel", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	panel.WriteTo(w)
}
