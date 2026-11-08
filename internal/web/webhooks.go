package web

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"slices"

	"github.com/Andrew-Sem/gc/internal/domain"
	"github.com/Andrew-Sem/gc/internal/github"
	"github.com/Andrew-Sem/gc/specs/contract"
)

const webhookBodyLimit = 25 << 20

var usedActions = []string{"opened", "reopened", "closed", "synchronize", "edited", "ready_for_review", "converted_to_draft"}

type pullRequestEvent struct {
	Action      string      `json:"action"`
	PullRequest github.Pull `json:"pull_request"`
	Repository  struct {
		FullName *string `json:"full_name"`
	} `json:"repository"`
}

func (e pullRequestEvent) snapshot() (contract.PrStateChanged, bool) {
	if e.Repository.FullName == nil {
		return contract.PrStateChanged{}, false
	}
	return e.PullRequest.Snapshot(normalize(*e.Repository.FullName))
}

func (s *server) receiveWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, webhookBodyLimit))
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		return
	}
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if !s.validSignature(r.Header.Get("X-Hub-Signature-256"), body) {
		http.Error(w, "Invalid signature", http.StatusUnauthorized)
		return
	}
	id, kind := r.Header.Get("X-GitHub-Delivery"), r.Header.Get("X-GitHub-Event")
	if id == "" || kind == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	s.webhooks.Lock()
	defer s.webhooks.Unlock()
	if s.deliveries[id] {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if kind != "pull_request" {
		s.deliveries[id] = true
		w.WriteHeader(http.StatusNoContent)
		return
	}
	var event pullRequestEvent
	if err := json.Unmarshal(body, &event); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	snapshot, complete := event.snapshot()
	if !complete {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	s.deliveries[id] = true
	facts := s.journal.Facts()
	if slices.Contains(usedActions, event.Action) && domain.IsOurPR(facts, snapshot.Repository, snapshot.Number) {
		record(s.journal, snapshot)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) validSignature(signature string, body []byte) bool {
	mac := hmac.New(sha256.New, []byte(s.webhookSecret))
	mac.Write(body)
	want := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(signature), []byte(want))
}
