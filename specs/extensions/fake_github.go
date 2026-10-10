package extensions

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
)

const fakeGitHubToken = "spec-github-token"

type fakeGitHub struct {
	server        *httptest.Server
	mu            sync.Mutex
	sees          []string
	asked         []string
	doesNotAnswer bool
}

func newFakeGitHub() *fakeGitHub {
	g := &fakeGitHub{}
	g.server = httptest.NewServer(http.HandlerFunc(g.serve))
	return g
}

func (g *fakeGitHub) serve(w http.ResponseWriter, r *http.Request) {
	fullName := strings.TrimPrefix(r.URL.Path, "/repos/")
	g.mu.Lock()
	g.asked = append(g.asked, fullName)
	sees, doesNotAnswer := slices.Contains(g.sees, fullName), g.doesNotAnswer
	g.mu.Unlock()

	if doesNotAnswer {
		panic(http.ErrAbortHandler)
	}
	w.Header().Set("Content-Type", "application/json")
	switch auth := r.Header.Get("Authorization"); {
	case auth == "":
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"message":"Not Found"}`)
	case auth != "Bearer "+fakeGitHubToken:
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"message":"Bad credentials"}`)
	case r.Method != http.MethodGet || !strings.HasPrefix(r.URL.Path, "/repos/") || !sees:
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"message":"Not Found"}`)
	default:
		fmt.Fprintf(w, `{"full_name":%q}`, fullName)
	}
}

func (g *fakeGitHub) askedSoFar() []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return slices.Clone(g.asked)
}

type githubAsked struct {
	got []string
}

func (p githubAsked) String() string {
	return fmt.Sprintf("expected GitHub not to be asked, got %v", p.got)
}
