package extensions

import (
	"bytes"
	"fmt"
	"maps"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
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
	covers  []string
	env     map[string]string
	unset   []string
	taken   bool
	request *spec.Request

	whenCalls     int
	expectExit    bool
	expectServing bool
	status        int
	outputs       []string
	absentOutputs []string
}

func newLaunch(binary string) *Launch {
	return &Launch{binary: binary, github: newFakeGitHub(), addr: freeAddress(), env: map[string]string{}}
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

func (l *Launch) Address() string {
	return l.addr
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
	return problems
}

func (l *Launch) environ() []string {
	env := map[string]string{
		"GC_ADDR":        l.addr,
		"GITHUB_API_URL": l.github.server.URL,
		"GITHUB_TOKEN":   fakeGitHubToken,
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
	if problems := l.shapeProblems(); len(problems) > 0 {
		return problems
	}
	if l.taken {
		taken, err := net.Listen("tcp", l.addr)
		if err != nil {
			return []spec.Problem{launchProblem(fmt.Sprintf("cannot take %s: %v", l.addr, err))}
		}
		defer taken.Close()
	}

	var output bytes.Buffer
	cmd := exec.Command(l.binary)
	cmd.Env = l.environ()
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Start(); err != nil {
		return []spec.Problem{launchProblem(fmt.Sprintf("cannot start gc: %v", err))}
	}
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()

	var problems []spec.Problem
	running := true
	if l.expectExit {
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
		ready, running = l.waitServing(exited)
		if !ready {
			problems = append(problems, launchProblem(fmt.Sprintf("expected gc to serve /repositories at %s within %s", l.addr, startupLimit)))
		} else if l.request != nil {
			problems = append(problems, l.send()...)
		}
	}

	if running {
		cmd.Process.Signal(os.Interrupt)
		select {
		case <-exited:
		case <-time.After(startupLimit):
			cmd.Process.Kill()
			<-exited
		}
	}

	for _, text := range l.outputs {
		if !strings.Contains(output.String(), text) {
			problems = append(problems, launchProblem(fmt.Sprintf("expected output to contain %q, got %q", text, output.String())))
		}
	}
	for _, text := range l.absentOutputs {
		if strings.Contains(output.String(), text) {
			problems = append(problems, launchProblem(fmt.Sprintf("expected output not to contain %q", text)))
		}
	}
	return problems
}

func (l *Launch) waitServing(exited <-chan error) (ready, running bool) {
	deadline := time.Now().Add(startupLimit)
	for time.Now().Before(deadline) {
		select {
		case <-exited:
			return false, false
		default:
		}
		if resp, err := http.Get("http://" + l.addr + "/repositories"); err == nil {
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
	resp.Body.Close()
	if resp.StatusCode != l.status {
		return []spec.Problem{launchProblem(fmt.Sprintf("expected status %d, got %d", l.status, resp.StatusCode))}
	}
	return nil
}
