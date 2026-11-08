package extensions

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
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
	fakeAppID            = "4242"
	fakeInstallationPath = "/app/installations/7/access_tokens"
	fakeWebhookSecret    = "spec-webhook-secret"
	fakeOpenAIKey        = "spec-openai-key"
	longerThanAnyTimeout = 2 * time.Minute
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
	createsPR     int
	createsAfter  time.Duration
	created       []createdPR
	quotesToken   string
	dropsFor      string
}

var fakeAppKey = sync.OnceValue(func() *rsa.PrivateKey {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}
	return key
})

func validAppToken(auth string) bool {
	token, found := strings.CutPrefix(auth, "Bearer ")
	parts := strings.Split(token, ".")
	if !found || len(parts) != 3 {
		return false
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return false
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if rsa.VerifyPKCS1v15(&fakeAppKey().PublicKey, crypto.SHA256, digest[:], signature) != nil {
		return false
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}
	var claims struct {
		Issuer    any   `json:"iss"`
		IssuedAt  int64 `json:"iat"`
		ExpiresAt int64 `json:"exp"`
	}
	if json.Unmarshal(raw, &claims) != nil || fmt.Sprint(claims.Issuer) != fakeAppID {
		return false
	}
	now := time.Now().Unix()
	return claims.IssuedAt <= now && now < claims.ExpiresAt && claims.ExpiresAt-claims.IssuedAt <= 600
}

func askedAbout(path string) (string, bool) {
	fullName, found := strings.CutPrefix(path, "/repos/")
	return strings.TrimSuffix(fullName, "/installation"), found
}

type createdPR struct {
	Repository  string
	Head        string `json:"head"`
	Base        string `json:"base"`
	Title       string `json:"title"`
	Description string `json:"body"`
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
	if fullName, found := askedAbout(req.URL.Path); unreachable && found {
		g.asked = append(g.asked, fullName)
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
	fullName, aboutRepository := askedAbout(r.URL.Path)
	g.mu.Lock()
	if aboutRepository {
		g.asked = append(g.asked, fullName)
	}
	sees := slices.Contains(g.sees, fullName)
	doesNotAnswer, rejectsToken, answer, hangs, echoesToken, moves, createsPR := g.doesNotAnswer, g.rejectsToken, g.answer, g.hangs, g.echoesToken, g.moves, g.createsPR
	quotesToken, dropsFor, createsAfter := g.quotesToken, g.dropsFor, g.createsAfter
	g.mu.Unlock()

	if quotesToken != "" {
		g.quoteToken(w, quotesToken)
		return
	}
	if dropsFor == fullName {
		panic(http.ErrAbortHandler)
	}
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
			message += " for Bearer " + fakeGitHubToken
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
	case rejectsToken:
		fail(http.StatusUnauthorized, "Bad credentials")
	case strings.HasSuffix(r.URL.Path, "/installation") || strings.HasPrefix(r.URL.Path, "/app/"):
		g.serveApp(w, r, fail, validAppToken(auth), sees || createsPR != 0)
	case auth != "Bearer "+fakeGitHubToken:
		fail(http.StatusUnauthorized, "Bad credentials")
	case r.Method == http.MethodPost && strings.HasSuffix(fullName, "/pulls") && createsPR != 0:
		var pr createdPR
		if err := json.NewDecoder(r.Body).Decode(&pr); err != nil || pr.Head == "" || pr.Base == "" || pr.Title == "" {
			fail(http.StatusUnprocessableEntity, "Validation Failed")
			return
		}
		select {
		case <-time.After(createsAfter):
		case <-r.Context().Done():
			return
		}
		pr.Repository = strings.TrimSuffix(fullName, "/pulls")
		g.mu.Lock()
		g.created = append(g.created, pr)
		g.mu.Unlock()
		w.WriteHeader(http.StatusCreated)
		fmt.Fprintf(w, `{"number":%d}`, createsPR)
	case r.Method != http.MethodGet || !strings.HasPrefix(r.URL.Path, "/repos/") || !sees:
		fail(http.StatusNotFound, "Not Found")
	default:
		fmt.Fprintf(w, `{"full_name":%q}`, canonicalCasing(fullName))
	}
}

func (g *fakeGitHub) serveApp(w http.ResponseWriter, r *http.Request, fail func(int, string), signed, installed bool) {
	switch {
	case !signed:
		fail(http.StatusUnauthorized, "A JSON web token could not be decoded")
	case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/installation") && installed:
		fmt.Fprint(w, `{"id":7}`)
	case r.Method == http.MethodPost && r.URL.Path == fakeInstallationPath:
		w.WriteHeader(http.StatusCreated)
		fmt.Fprintf(w, `{"token":%q,"expires_at":%q}`, fakeGitHubToken, time.Now().Add(time.Hour).UTC().Format(time.RFC3339))
	default:
		fail(http.StatusNotFound, "Not Found")
	}
}

func (g *fakeGitHub) quoteToken(w http.ResponseWriter, form string) {
	conn, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	fmt.Fprintf(conn, form, fakeGitHubToken)
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

type githubPRCreated struct {
	want createdPR
	got  []createdPR
}

func (p githubPRCreated) String() string {
	return fmt.Sprintf("expected GitHub to create only the PR %+v, got %+v", p.want, p.got)
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
