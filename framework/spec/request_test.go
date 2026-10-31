package spec

import (
	"io"
	"slices"
	"testing"
)

func TestHTTPRequestTargetsBaseURL(t *testing.T) {
	r, err := Post("/repositories").WithForm("repo", "acme/api").HTTPRequest("http://127.0.0.1:9999")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(r.Body)
	got := []string{r.Method, r.URL.String(), r.Header.Get("Content-Type"), string(body)}
	want := []string{"POST", "http://127.0.0.1:9999/repositories", "application/x-www-form-urlencoded", "repo=acme%2Fapi"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("request = %q, want %q", got, want)
			break
		}
	}
}

func TestHTTPRequestRejectsBadBaseURL(t *testing.T) {
	if _, err := Get("/repositories").HTTPRequest("http://bad host"); err == nil {
		t.Error("want an error for a malformed base URL")
	}
}

func TestHTTPRequestCarriesHeadersAndBody(t *testing.T) {
	r, err := Post("/webhooks/github").
		WithHeader("X-GitHub-Event", "pull_request").
		WithBody("application/json", `{"action":"opened"}`).
		HTTPRequest("http://127.0.0.1:9999")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(r.Body)
	got := []string{r.Header.Get("X-GitHub-Event"), r.Header.Get("Content-Type"), string(body)}
	want := []string{"pull_request", "application/json", `{"action":"opened"}`}
	if !slices.Equal(got, want) {
		t.Errorf("request = %q, want %q", got, want)
	}
}

func TestInMemoryRequestCarriesHeadersAndBody(t *testing.T) {
	r := Post("/webhooks/github").
		WithHeader("X-GitHub-Event", "pull_request").
		WithBody("application/json", `{"action":"opened"}`).
		httpRequest()
	body, _ := io.ReadAll(r.Body)
	got := []string{r.Header.Get("X-GitHub-Event"), r.Header.Get("Content-Type"), string(body)}
	want := []string{"pull_request", "application/json", `{"action":"opened"}`}
	if !slices.Equal(got, want) {
		t.Errorf("request = %q, want %q", got, want)
	}
}

func TestWithHeaderLeavesOriginalUnchanged(t *testing.T) {
	original := Post("/webhooks/github").WithHeader("X-GitHub-Event", "pull_request")
	original.WithHeader("X-GitHub-Event", "push")
	if got := original.httpRequest().Header.Get("X-GitHub-Event"); got != "pull_request" {
		t.Errorf("original event = %q, want %q", got, "pull_request")
	}
}

func TestWithoutHeaderRemovesOnlyThatHeader(t *testing.T) {
	original := Post("/webhooks/github").
		WithHeader("X-GitHub-Event", "pull_request").
		WithHeader("X-Hub-Signature-256", "sha256=abc")
	got := original.WithoutHeader("X-Hub-Signature-256").httpRequest().Header
	if _, present := got["X-Hub-Signature-256"]; present {
		t.Errorf("signature header is still there: %v", got)
	}
	if got.Get("X-GitHub-Event") != "pull_request" {
		t.Errorf("event header = %q, want %q", got.Get("X-GitHub-Event"), "pull_request")
	}
	if original.httpRequest().Header.Get("X-Hub-Signature-256") != "sha256=abc" {
		t.Error("original request lost its header")
	}
}

func TestWithoutHeaderOnRequestWithoutHeaders(t *testing.T) {
	got := Post("/webhooks/github").WithoutHeader("X-Hub-Signature-256").httpRequest().Header
	if _, present := got["X-Hub-Signature-256"]; present {
		t.Errorf("signature header appeared: %v", got)
	}
}
