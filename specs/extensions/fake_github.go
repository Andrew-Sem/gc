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
	rejectsToken  bool
	answer        *cannedAnswer
}

type cannedAnswer struct {
	status int
	header []string
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
	sees, doesNotAnswer, rejectsToken, answer := slices.Contains(g.sees, fullName), g.doesNotAnswer, g.rejectsToken, g.answer
	g.mu.Unlock()

	if doesNotAnswer {
		panic(http.ErrAbortHandler)
	}
	w.Header().Set("Content-Type", "application/json")
	if answer != nil {
		for i := 0; i+1 < len(answer.header); i += 2 {
			w.Header().Set(answer.header[i], answer.header[i+1])
		}
		w.WriteHeader(answer.status)
		fmt.Fprintf(w, `{"message":%q}`, http.StatusText(answer.status))
		return
	}
	switch auth := r.Header.Get("Authorization"); {
	case auth == "":
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"message":"Not Found"}`)
	case auth != "Bearer "+fakeGitHubToken || rejectsToken:
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
