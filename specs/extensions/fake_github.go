package extensions

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"time"
)

const (
	movedRepositoryPath  = "/repositories/42"
	fakeGitHubURL        = "http://127.0.0.1:9"
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
	unreachable   bool
	moves         int
}

type cannedAnswer struct {
	status int
	header []string
}

func newFakeGitHub() *fakeGitHub {
	return &fakeGitHub{hangEnded: make(chan struct{})}
}

func (g *fakeGitHub) listen() string {
	g.server = httptest.NewServer(http.HandlerFunc(g.serve))
	return g.server.URL
}

func (g *fakeGitHub) transport() http.RoundTripper {
	return inMemory{github: g}
}

type inMemory struct {
	github *fakeGitHub
}

func (t inMemory) RoundTrip(req *http.Request) (*http.Response, error) {
	g := t.github
	g.mu.Lock()
	unreachable := g.unreachable
	if unreachable {
		g.asked = append(g.asked, strings.TrimPrefix(req.URL.Path, "/repos/"))
	}
	g.mu.Unlock()
	if unreachable {
		return nil, &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")}
	}

	ctx, cancel := context.WithCancel(req.Context())
	defer cancel()
	response := httptest.NewRecorder()
	aborted := make(chan bool, 1)
	go func() {
		defer func() { aborted <- recover() != nil }()
		http.HandlerFunc(g.serve).ServeHTTP(response, req.WithContext(ctx))
	}()
	select {
	case <-req.Context().Done():
		cancel()
		<-aborted
		return nil, req.Context().Err()
	case wasAborted := <-aborted:
		if wasAborted {
			return nil, errors.New("connection closed by GitHub")
		}
		return response.Result(), nil
	}
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
	doesNotAnswer, rejectsToken, answer, hangs, echoesToken, moves := g.doesNotAnswer, g.rejectsToken, g.answer, g.hangs, g.echoesToken, g.moves
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
	case moves != 0 && strings.HasPrefix(r.URL.Path, "/repos/"):
		w.Header().Set("Location", movedRepositoryPath)
		fail(moves, http.StatusText(moves))
	case moves != 0 && r.URL.Path == movedRepositoryPath:
		fmt.Fprint(w, `{"full_name":"Acme/Api-Renamed"}`)
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
		fmt.Fprintf(w, `{"full_name":%q}`, canonicalCasing(fullName))
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
	want, got []string
}

func (p githubAsked) String() string {
	if len(p.want) == 0 {
		return fmt.Sprintf("expected GitHub not to be asked, got %v", p.got)
	}
	return fmt.Sprintf("expected GitHub to be asked about %v, got %v", p.want, p.got)
}

type githubWaited struct {
	want, got time.Duration
}

func (p githubWaited) String() string {
	return fmt.Sprintf("expected the application to wait %s for GitHub, it waited %s", p.want, p.got.Round(time.Millisecond))
}

func canonicalCasing(fullName string) string {
	owner, name, _ := strings.Cut(fullName, "/")
	return strings.ToUpper(owner[:1]) + owner[1:] + "/" + strings.ToUpper(name[:1]) + name[1:]
}
