package spec

import (
	"io"
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
