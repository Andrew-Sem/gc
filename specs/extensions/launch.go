package extensions

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"maps"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Andrew-Sem/gc/framework/spec"
)

const (
	gcPackage    = "github.com/Andrew-Sem/gc/cmd/gc"
	startupLimit = 5 * time.Second
)

type Launch struct {
	binary  string
	github  *fakeGitHub
	addr    string
	apiURL  string
	proxy   *fakeProxy
	covers  []string
	env     map[string]string
	unset   []string
	taken   bool
	request *spec.Request
	waitRun bool
	pause   time.Duration

	whenCalls     int
	expectExit    bool
	expectServing bool
	status        int
	outputs       []string
	absentOutputs []string
	proxyAsked    []string
	stream        *launchStream
	asked         []string
	requests      []spec.Request

	journalFile
}

func newLaunch(binary string) *Launch {
	github := newFakeGitHub()
	return &Launch{
		binary:      binary,
		github:      github,
		apiURL:      github.listen(),
		proxy:       newFakeProxy(),
		addr:        freeAddress(),
		env:         map[string]string{},
		journalFile: newJournalFile(),
	}
}

func freeAddress() string {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	defer l.Close()
	return l.Addr().String()
}

func (f *Feature) RunLaunch(name string, describe func(*Launch)) {
	f.t.Helper()
	binary := f.gcBinary()
	f.core.RunCustom(name, func() spec.Custom {
		l := newLaunch(binary)
		describe(l)
		return l
	})
}

func (f *Feature) gcBinary() string {
	if f.binary == "" {
		path := filepath.Join(f.t.TempDir(), "gc")
		if out, err := exec.Command("go", "build", "-o", path, gcPackage).CombinedOutput(); err != nil {
			f.t.Fatalf("cannot build %s: %v\n%s", gcPackage, err, out)
		}
		f.binary = path
	}
	return f.binary
}

func (l *Launch) Covers(ids ...string) *Launch {
	l.covers = ids
	return l
}

func (l *Launch) GivenEnv(name, value string) *Launch {
	l.env[name] = value
	return l
}

func (l *Launch) GivenUnset(name string) *Launch {
	l.unset = append(l.unset, name)
	return l
}

func (l *Launch) GivenAddressTaken() *Launch {
	l.taken = true
	return l
}

func (l *Launch) GivenGitHubSees(fullNames ...string) *Launch {
	l.github.given(func(g *fakeGitHub) { g.sees = fullNames })
	return l
}

func (l *Launch) GivenGitHubAnswers(status int, header ...string) *Launch {
	l.github.given(func(g *fakeGitHub) { g.answer = &cannedAnswer{status: status, header: header} })
	return l
}

func (l *Launch) GivenGitHubQuotesTokenInMalformedAnswer() *Launch {
	l.github.given(func(g *fakeGitHub) { g.quotesToken = QuotedInStatus })
	return l
}

func (l *Launch) GivenGitHubDropsConnectionFor(fullName string) *Launch {
	l.github.given(func(g *fakeGitHub) { g.dropsFor = fullName })
	return l
}

func (l *Launch) GivenRequest(r spec.Request) *Launch {
	l.requests = append(l.requests, r)
	return l
}

func (l *Launch) ExpectGitHubAsked(fullNames ...string) *Launch {
	l.asked = fullNames
	return l
}

func (l *Launch) GivenGitHubEchoesToken() *Launch {
	l.github.given(func(g *fakeGitHub) { g.echoesToken = true })
	return l
}

func (l *Launch) WhenStarted() *Launch {
	l.whenCalls++
	return l
}

func (l *Launch) When(r spec.Request) *Launch {
	l.request = &r
	l.whenCalls++
	return l
}

func (l *Launch) Then(description string) *Launch {
	return l
}

func (l *Launch) ExpectNoSuccessForTheWrite() *Launch {
	l.expectNoSuccess = true
	return l
}

func (l *Launch) ExpectExit() *Launch {
	l.expectExit = true
	return l
}

func (l *Launch) ExpectServing() *Launch {
	l.expectServing = true
	return l
}

func (l *Launch) ExpectStatus(status int) *Launch {
	l.status = status
	return l
}

func (l *Launch) ExpectOutput(text string) *Launch {
	l.outputs = append(l.outputs, text)
	return l
}

func (l *Launch) ExpectOutputWithout(text string) *Launch {
	l.absentOutputs = append(l.absentOutputs, text)
	return l
}

func (l *Launch) ExpectProxyAsked(hosts ...string) *Launch {
	l.proxyAsked = hosts
	return l
}

func (l *Launch) Address() string {
	return l.addr
}

func (l *Launch) WebhookSecret() string {
	return fakeWebhookSecret
}

func (l *Launch) keyFile() string {
	path := filepath.Join(l.dir, "gc-app.pem")
	if _, err := os.Stat(path); err == nil {
		return path
	}
	block := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(fakeAppKey())}
	if err := os.WriteFile(path, pem.EncodeToMemory(block), 0o600); err != nil {
		panic(err)
	}
	return path
}

func (l *Launch) PrivateKeyFile() string {
	return l.keyFile()
}

func (l *Launch) PrivateKeyText() string {
	block := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(fakeAppKey())}
	return strings.Split(string(pem.EncodeToMemory(block)), "\n")[2]
}

func (l *Launch) GitHubToken() string {
	return fakeGitHubToken
}

func (l *Launch) Covered() []string {
	return l.covers
}

type launchProblem string

func (p launchProblem) String() string {
	return string(p)
}

func (l *Launch) shapeProblems() []spec.Problem {
	var problems []spec.Problem
	if l.whenCalls != 1 {
		problems = append(problems, launchProblem(fmt.Sprintf("expected one When or WhenStarted call, got %d", l.whenCalls)))
	}
	serving := l.expectServing || l.status != 0
	if l.expectExit == serving {
		problems = append(problems, launchProblem("expected either ExpectExit or ExpectServing/ExpectStatus"))
	}
	if l.request != nil && l.status == 0 {
		problems = append(problems, launchProblem("expected ExpectStatus for the request"))
	}
	if l.request == nil && len(l.elements)+len(l.noElements) > 0 {
		problems = append(problems, launchProblem("expected a request for ExpectElement or ExpectNoElement"))
	}
	if l.failingWrite != nil && !l.expectExit {
		problems = append(problems, launchProblem("expected ExpectExit for a failing write"))
	}
	return problems
}

func (l *Launch) environ() []string {
	env := map[string]string{
		"GC_ADDR":                     l.addr,
		"GC_DB":                       l.path,
		"GC_WEBHOOK_SECRET":           fakeWebhookSecret,
		"GITHUB_API_URL":              l.apiURL,
		"GITHUB_APP_ID":               fakeAppID,
		"GITHUB_APP_PRIVATE_KEY_FILE": l.keyFile(),
		"HTTPS_PROXY":                 l.proxy.server.URL,
		"HTTP_PROXY":                  l.proxy.server.URL,
		"NO_PROXY":                    "",
		"OPENAI_API_KEY":              fakeOpenAIKey,
		"no_proxy":                    "",
	}
	maps.Copy(env, l.env)
	var result []string
	for _, pair := range os.Environ() {
		name, _, _ := strings.Cut(pair, "=")
		if _, ours := env[name]; !ours && !slices.Contains(l.unset, name) {
			result = append(result, pair)
		}
	}
	for _, name := range slices.Sorted(maps.Keys(env)) {
		if !slices.Contains(l.unset, name) {
			result = append(result, name+"="+env[name])
		}
	}
	return result
}

func (l *Launch) Run() []spec.Problem {
	defer l.github.server.Close()
	defer l.proxy.server.Close()
	defer l.removeJournalFile()
	defer l.closeStream()
	if problems := l.shapeProblems(); len(problems) > 0 {
		return problems
	}
	if err := l.prepareJournalFile(); err != nil {
		return []spec.Problem{launchProblem(fmt.Sprintf("cannot prepare the journal file: %v", err))}
	}
	for _, earlier := range l.earlier {
		if problem := l.startEarlier(earlier); problem != nil {
			return []spec.Problem{problem}
		}
	}
	var other *process
	if l.otherRunning {
		var problem spec.Problem
		if other, problem = l.startOther(); problem != nil {
			return []spec.Problem{problem}
		}
		defer other.stop()
	}
	if l.taken {
		taken, err := net.Listen("tcp", l.addr)
		if err != nil {
			return []spec.Problem{launchProblem(fmt.Sprintf("cannot take %s: %v", l.addr, err))}
		}
		defer taken.Close()
	}

	main, err := l.start()
	if err != nil {
		return []spec.Problem{launchProblem(fmt.Sprintf("cannot start gc: %v", err))}
	}
	output, cmd, exited := &main.output, main.cmd, main.exited

	var problems []spec.Problem
	running := true
	if l.expectExit {
		if l.failingWrite != nil {
			problems = append(problems, l.breakWrites(exited)...)
		}
		select {
		case err := <-exited:
			running = false
			if err == nil {
				problems = append(problems, launchProblem("expected gc to exit with an error, it exited successfully"))
			}
		case <-time.After(startupLimit):
			problems = append(problems, launchProblem(fmt.Sprintf("expected gc to exit, it was still running after %s", startupLimit)))
		}
	} else {
		var ready bool
		ready, running = waitServing(l.addr, exited)
		if !ready {
			problems = append(problems, launchProblem(fmt.Sprintf("expected gc to serve /repositories at %s within %s", l.addr, startupLimit)))
		} else if l.request != nil {
			for _, request := range l.requests {
				if _, err := l.do(l.addr, request); err != nil {
					problems = append(problems, launchProblem(fmt.Sprintf("cannot send the earlier request: %v", err)))
				}
			}
			problems = append(problems, l.send()...)
			if l.waitRun {
				problems = append(problems, l.waitRunEnd()...)
			}
			time.Sleep(l.pause)
		} else if l.stream != nil {
			problems = append(problems, l.watchStream()...)
		}
	}

	if running {
		cmd.Process.Signal(os.Interrupt)
		select {
		case <-exited:
		case <-time.After(startupLimit):
			cmd.Process.Kill()
			<-exited
			if l.stream != nil {
				problems = append(problems, launchProblem(fmt.Sprintf("expected gc to stop within %s with a stream open", startupLimit)))
			}
		}
	}

	for _, text := range l.outputs {
		if !strings.Contains(output.String(), text) {
			problems = append(problems, launchProblem(fmt.Sprintf("expected output to contain %q, got %q", text, output.String())))
		}
	}
	if l.asked != nil {
		if got := l.github.askedSoFar(); !slices.Equal(got, l.asked) {
			problems = append(problems, launchProblem(fmt.Sprintf("expected GitHub to be asked about %v, got %v", l.asked, got)))
		}
	}
	if l.proxyAsked != nil {
		if got := l.proxy.askedSoFar(); !slices.Equal(got, l.proxyAsked) {
			problems = append(problems, launchProblem(fmt.Sprintf("expected the proxy to be asked for %v, got %v", l.proxyAsked, got)))
		}
	}
	for _, text := range l.absentOutputs {
		if strings.Contains(output.String(), text) {
			problems = append(problems, launchProblem(fmt.Sprintf("expected output not to contain %q", text)))
		}
	}
	if other != nil {
		problems = append(problems, l.otherStillWorks(other)...)
	}
	return append(problems, l.journalFileProblems()...)
}

func waitServing(addr string, exited <-chan error) (ready, running bool) {
	deadline := time.Now().Add(startupLimit)
	for time.Now().Before(deadline) {
		select {
		case <-exited:
			return false, false
		default:
		}
		if resp, err := http.Get("http://" + addr + "/repositories"); err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return true, true
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false, true
}

func (l *Launch) send() []spec.Problem {
	req, err := l.request.HTTPRequest("http://" + l.addr)
	if err != nil {
		return []spec.Problem{launchProblem(fmt.Sprintf("cannot build the request: %v", err))}
	}
	client := &http.Client{Timeout: 2 * longerThanAnyTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return []spec.Problem{launchProblem(fmt.Sprintf("cannot send the request: %v", err))}
	}
	defer resp.Body.Close()
	if resp.StatusCode != l.status {
		return []spec.Problem{launchProblem(fmt.Sprintf("expected status %d, got %d", l.status, resp.StatusCode))}
	}
	return l.pageProblems(resp.Body)
}

type fakeProxy struct {
	server *httptest.Server
	mu     sync.Mutex
	asked  []string
}

func newFakeProxy() *fakeProxy {
	p := &fakeProxy{}
	p.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.mu.Lock()
		p.asked = append(p.asked, r.Host)
		p.mu.Unlock()
		http.Error(w, "the spec proxy forwards nothing", http.StatusForbidden)
	}))
	return p
}

func (p *fakeProxy) askedSoFar() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.asked)
}

var ECPrivateKey, ECPrivateKeyLine = func() (string, string) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		panic(err)
	}
	text := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
	return text, strings.Split(text, "\n")[1]
}()
