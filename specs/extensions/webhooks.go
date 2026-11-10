package extensions

import (
	"cmp"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Andrew-Sem/gc/framework/spec"
)

type PullRequest struct {
	Action     string
	Repository string
	Number     int
	State      string
	Merged     bool
	HeadSHA    string
	Base       string
	Draft      bool
	Mergeable  string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

var deliveries atomic.Int64

func PullRequestDelivery(pr PullRequest) spec.Request {
	return RawDelivery("pull_request", marshal(pullRequestPayload(pr)))
}

func PullRequestDeliveryWithout(field string) spec.Request {
	payload := pullRequestPayload(PullRequest{Action: "synchronize", Repository: "acme/api", Number: 7})
	path := strings.Split(field, ".")
	parent := payload
	for _, key := range path[:len(path)-1] {
		parent = parent[key].(map[string]any)
	}
	delete(parent, path[len(path)-1])
	return RawDelivery("pull_request", marshal(payload))
}

func RawDelivery(event, body string) spec.Request {
	return spec.Post("/webhooks/github").
		WithHeader("X-GitHub-Event", event).
		WithHeader("X-GitHub-Delivery", nextDeliveryID()).
		WithHeader("X-Hub-Signature-256", signature([]byte(body))).
		WithBody("application/json", body)
}

func pullRequestPayload(pr PullRequest) map[string]any {
	createdAt := pr.CreatedAt
	if createdAt.IsZero() {
		createdAt = At("1999-12-31 08:00")
	}
	updatedAt := pr.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = At("1999-12-31 10:00")
	}
	return map[string]any{
		"action": pr.Action,
		"number": pr.Number,
		"pull_request": map[string]any{
			"number":     pr.Number,
			"state":      cmp.Or(pr.State, "open"),
			"merged":     pr.Merged,
			"draft":      pr.Draft,
			"mergeable":  json.RawMessage(cmp.Or(pr.Mergeable, "true")),
			"created_at": createdAt.Format(time.RFC3339),
			"updated_at": updatedAt.Format(time.RFC3339),
			"head":       map[string]any{"sha": cmp.Or(pr.HeadSHA, "abc123")},
			"base":       map[string]any{"ref": cmp.Or(pr.Base, "main")},
		},
		"repository": map[string]any{"full_name": pr.Repository},
	}
}

func marshal(payload map[string]any) string {
	body, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	return string(body)
}

func signature(body []byte) string {
	mac := hmac.New(sha256.New, []byte(fakeWebhookSecret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func nextDeliveryID() string {
	return "delivery-" + strconv.FormatInt(deliveries.Add(1), 10)
}

type CheckRunEvent struct {
	Action      string
	Repository  string
	Pulls       []int
	ID          int64
	Name        string
	AppID       int64
	HeadSHA     string
	Status      string
	Conclusion  string
	StartedAt   time.Time
	CompletedAt time.Time
}

func CheckRunDelivery(event CheckRunEvent) spec.Request {
	return RawDelivery("check_run", marshal(checkRunPayload(event)))
}

func CheckRunDeliveryWithout(event CheckRunEvent, field string) spec.Request {
	payload := checkRunPayload(event)
	path := strings.Split(field, ".")
	parent := payload
	for _, key := range path[:len(path)-1] {
		parent = parent[key].(map[string]any)
	}
	delete(parent, path[len(path)-1])
	return RawDelivery("check_run", marshal(payload))
}

func checkRunPayload(event CheckRunEvent) map[string]any {
	pulls := []any{}
	for _, number := range event.Pulls {
		pulls = append(pulls, map[string]any{
			"number": number,
			"head":   map[string]any{"sha": event.HeadSHA},
			"base":   map[string]any{"ref": "main"},
		})
	}
	var conclusion, completedAt any
	if event.Conclusion != "" {
		conclusion = event.Conclusion
	}
	if !event.CompletedAt.IsZero() {
		completedAt = event.CompletedAt.Format(time.RFC3339)
	}
	return map[string]any{
		"action": event.Action,
		"check_run": map[string]any{
			"id":            event.ID,
			"name":          event.Name,
			"app":           map[string]any{"id": event.AppID},
			"head_sha":      event.HeadSHA,
			"status":        event.Status,
			"conclusion":    conclusion,
			"started_at":    event.StartedAt.Format(time.RFC3339),
			"completed_at":  completedAt,
			"pull_requests": pulls,
		},
		"repository": map[string]any{"full_name": event.Repository},
	}
}
