package extensions

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"time"
)

const (
	fakeGitHubToken      = "spec-github-token"
	longerThanAnyTimeout = 10 * time.Second
)

type fakeGitHub struct {
	server        *httptest.Server
	mu            sync.Mutex
	sees          []string
	asked         []string
	doesNotAnswer bool
	rejectsToken  bool
	answer        *cannedAnswer
	hangs         bool
	waited        time.Duration
	hangEnded     chan struct{}
	echoesToken   bool
}

type cannedAnswer struct {
	status int
	header []string
}

func newFakeGitHub() *fakeGitHub {
	g := &fakeGitHub{hangEnded: make(chan struct{})}
	g.server = httptest.NewServer(http.HandlerFunc(g.serve))
	return g
}

func (g *fakeGitHub) given(change func(*fakeGitHub)) {
	g.mu.Lock()
	defer g.mu.Unlock()
	change(g)
}

func (g *fakeGitHub) serve(w http.ResponseWriter, r *http.Request) {
	fullName := strings.TrimPrefix(r.URL.Path, "/repos/")
	g.mu.Lock()
	g.asked = append(g.asked, fullName)
	sees := slices.Contains(g.sees, fullName)
	doesNotAnswer, rejectsToken, answer, hangs, echoesToken := g.doesNotAnswer, g.rejectsToken, g.answer, g.hangs, g.echoesToken
	g.mu.Unlock()

	if hangs {
		g.hang(r)
	}
	if doesNotAnswer {
		panic(http.ErrAbortHandler)
	}

	auth := r.Header.Get("Authorization")
	w.Header().Set("Content-Type", "application/json")
	fail := func(status int, message string) {
		w.WriteHeader(status)
		if echoesToken {
			message += " for " + auth
		}
		fmt.Fprintf(w, `{"message":%q}`, message)
	}
	switch {
	case answer != nil:
		for i := 0; i+1 < len(answer.header); i += 2 {
			w.Header().Set(answer.header[i], answer.header[i+1])
		}
		fail(answer.status, http.StatusText(answer.status))
	case auth == "":
		fail(http.StatusNotFound, "Not Found")
	case auth != "Bearer "+fakeGitHubToken || rejectsToken:
		fail(http.StatusUnauthorized, "Bad credentials")
	case r.Method != http.MethodGet || !strings.HasPrefix(r.URL.Path, "/repos/") || !sees:
		fail(http.StatusNotFound, "Not Found")
	default:
		fmt.Fprintf(w, `{"full_name":%q}`, fullName)
	}
}

func (g *fakeGitHub) hang(r *http.Request) {
	start := time.Now()
	select {
	case <-r.Context().Done():
	case <-time.After(longerThanAnyTimeout):
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.waited == 0 {
		g.waited = time.Since(start)
		close(g.hangEnded)
	}
}

func (g *fakeGitHub) waitedSoFar() time.Duration {
	select {
	case <-g.hangEnded:
	case <-time.After(time.Second):
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.waited
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

type githubWaited struct {
	want, got time.Duration
}

func (p githubWaited) String() string {
	return fmt.Sprintf("expected the application to wait %s for GitHub, it waited %s", p.want, p.got.Round(time.Millisecond))
}
