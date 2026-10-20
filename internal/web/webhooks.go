package web

import (
	"encoding/json"
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
	var event pullRequestEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	s.journal.Append(contract.PrStateChanged{
		Repository: event.Repository.FullName,
		Number:     event.PullRequest.Number,
	})
	w.WriteHeader(http.StatusNoContent)
}
