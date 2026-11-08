package web

import (
	"bytes"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Andrew-Sem/gc/internal/domain"
	"github.com/Andrew-Sem/gc/specs/contract"
)

type settingsForm struct {
	Repository string
	Pace       string
	Limit      string
	Saved      bool
	Problem    string
}

func (s *server) settingsPage(w http.ResponseWriter, r *http.Request) {
	fullName := normalize(r.PathValue("owner") + "/" + r.PathValue("name"))
	facts := s.journal.Facts()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if !domain.IsConnected(facts, fullName) {
		http.Error(w, "This repository is not connected", http.StatusNotFound)
		return
	}
	settings := domain.RepositorySettings(facts, fullName)
	form := settingsForm{Repository: fullName, Pace: strconv.Itoa(settings.Pace), Limit: strconv.Itoa(settings.Limit)}
	if err := settingsView(form).Render(r.Context(), w); err != nil {
		http.Error(w, "cannot render the page", http.StatusInternalServerError)
	}
}

func (s *server) saveSettings(w http.ResponseWriter, r *http.Request) {
	form := settingsForm{
		Repository: normalize(r.PathValue("owner") + "/" + r.PathValue("name")),
		Pace:       strings.TrimSpace(r.FormValue("pace")),
		Limit:      strings.TrimSpace(r.FormValue("limit")),
	}
	facts := s.journal.Facts()
	pace, paceOK := wholeNumber(form.Pace, domain.MaxPace)
	limit, limitOK := wholeNumber(form.Limit, domain.MaxLimit)
	switch {
	case !domain.IsConnected(facts, form.Repository):
		form.Problem = "This repository is not connected"
	case !paceOK:
		form.Problem = "Pace must be a whole number of hours from 1 to 168"
	case !limitOK:
		form.Problem = "Maximum open must be a whole number from 1 to 100"
	}
	if form.Problem != "" {
		s.renderSettings(w, r, http.StatusUnprocessableEntity, form)
		return
	}
	if !domain.RepositorySettings(facts, form.Repository).Same(pace, limit) {
		s.journal.Append(contract.RepositorySettingsChanged{Repository: form.Repository, Pace: pace, Limit: limit, At: time.Now().UTC()})
	}
	form.Pace, form.Limit, form.Saved = strconv.Itoa(pace), strconv.Itoa(limit), true
	s.renderSettings(w, r, http.StatusOK, form)
}

func wholeNumber(text string, max int) (int, bool) {
	n, err := strconv.Atoi(text)
	return n, err == nil && n >= 1 && n <= max
}

func (s *server) renderSettings(w http.ResponseWriter, r *http.Request, status int, form settingsForm) {
	var panel bytes.Buffer
	if err := settingsPanel(form).Render(r.Context(), &panel); err != nil {
		http.Error(w, "cannot render the panel", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	panel.WriteTo(w)
}
