package web

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"

	"github.com/Andrew-Sem/gc/specs/contract"
)

type pullRequestEvent struct {
	PullRequest struct {
		Number int `json:"number"`
	} `json:"pull_request"`
	Repository struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
}

func (s *server) receiveWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if !s.validSignature(r.Header.Get("X-Hub-Signature-256"), body) {
		http.Error(w, "Invalid signature", http.StatusUnauthorized)
		return
	}
	var event pullRequestEvent
	if err := json.Unmarshal(body, &event); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	s.journal.Append(contract.PrStateChanged{
		Repository: event.Repository.FullName,
		Number:     event.PullRequest.Number,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) validSignature(signature string, body []byte) bool {
	mac := hmac.New(sha256.New, []byte(s.webhookSecret))
	mac.Write(body)
	want := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(signature), []byte(want))
}
