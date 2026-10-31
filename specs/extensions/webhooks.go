package extensions

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"sync/atomic"

	"github.com/Andrew-Sem/gc/framework/spec"
)

type PullRequest struct {
	Action     string
	Repository string
	Number     int
	State      string
}

type pullRequestPayload struct {
	Action      string            `json:"action"`
	Number      int               `json:"number"`
	PullRequest pullRequestObject `json:"pull_request"`
	Repository  repositoryObject  `json:"repository"`
}

type pullRequestObject struct {
	Number int    `json:"number"`
	State  string `json:"state"`
}

type repositoryObject struct {
	FullName string `json:"full_name"`
}

var deliveries atomic.Int64

func PullRequestDelivery(pr PullRequest) spec.Request {
	body, err := json.Marshal(pullRequestPayload{
		Action:      pr.Action,
		Number:      pr.Number,
		PullRequest: pullRequestObject{Number: pr.Number, State: pr.State},
		Repository:  repositoryObject{FullName: pr.Repository},
	})
	if err != nil {
		panic(err)
	}
	return spec.Post("/webhooks/github").
		WithHeader("X-GitHub-Event", "pull_request").
		WithHeader("X-GitHub-Delivery", nextDeliveryID()).
		WithHeader("X-Hub-Signature-256", signature(body)).
		WithBody("application/json", string(body))
}

func signature(body []byte) string {
	mac := hmac.New(sha256.New, []byte(fakeWebhookSecret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func nextDeliveryID() string {
	return "delivery-" + strconv.FormatInt(deliveries.Add(1), 10)
}
