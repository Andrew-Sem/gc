package extensions

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"net/http"
	"net/http/cgi"
	"net/http/httptest"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Andrew-Sem/gc/framework/spec"
	"github.com/Andrew-Sem/gc/internal/agent"
	"github.com/Andrew-Sem/gc/specs/contract"
)

const (
	agentRepository = "acme/api"
	agentRunLimit   = 30 * time.Second
	processGoneWait = 5 * time.Second
)

type AgentSaw struct {
	Commits            int
	Branch             string
	Prompt             bool
	InWorkingCopy      bool
	EmptyHome          bool
	OwnTemporaryFolder bool
}

type AgentRun struct {
	covers []string
	env    map[string]string
	unset  []string
	checks []func() []spec.Problem

	commits      int
	noRepository bool
	script       []string
	finalScript  []string
	whens        int
	runs         int
	stopped      bool
	expectFails  bool

	gitAuthenticated map[string]string

	remote  string
	records string
	change  *contract.AgentChange
	err     error
}

func (f *Feature) RunAgent(name string, describe func(*AgentRun)) {
	f.t.Helper()
	f.core.RunCustom(name, func() spec.Custom {
		a := &AgentRun{env: map[string]string{
			"GITHUB_TOKEN":      fakeGitHubToken,
			"GC_WEBHOOK_SECRET": fakeWebhookSecret,
			"OPENAI_API_KEY":    fakeOpenAIKey,
		}}
		describe(a)
		return a
	})
}

func (a *AgentRun) Covers(ids ...string) *AgentRun {
	a.covers = ids
	return a
}

func (a *AgentRun) Covered() []string {
	return a.covers
}

func (a *AgentRun) GivenEnv(name, value string) *AgentRun {
	a.env[name] = value
	return a
}

func (a *AgentRun) GivenUnset(name string) *AgentRun {
	a.unset = append(a.unset, name)
	return a
}

func (a *AgentRun) GivenRepositoryWithCommits(count int) *AgentRun {
	a.commits = count
	return a
}

func (a *AgentRun) GivenNoRepository() *AgentRun {
	a.noRepository = true
	return a
}

func (a *AgentRun) GivenAgentChanges(file, content string) *AgentRun {
	a.script = append(a.script, fmt.Sprintf("printf '%%s' %s > %s", shellQuote(content), shellQuote(file)))
	return a
}

func (a *AgentRun) GivenAgentAnswers(answer string) *AgentRun {
	a.script = append(a.script, fmt.Sprintf(`printf '%%s' %s > "$answer"`, shellQuote(answer)))
	return a
}

func (a *AgentRun) GivenAgentReadsInput() *AgentRun {
	a.script = append(a.script, "cat > /dev/null")
	return a
}

func (a *AgentRun) GivenAgentExitsWithError() *AgentRun {
	a.finalScript = append(a.finalScript, "exit 3")
	return a
}

func (a *AgentRun) GivenAgentHangsWithChildProcess() *AgentRun {
	a.finalScript = append(a.finalScript, `sleep 120 &
echo $! > "$records/child"
touch "$records/hanging"
wait`)
	return a
}

func (a *AgentRun) WhenAgentRuns() *AgentRun {
	a.whens++
	a.runs = 1
	return a
}

func (a *AgentRun) WhenAgentRunsTwice() *AgentRun {
	a.whens++
	a.runs = 2
	return a
}

func (a *AgentRun) WhenAgentRunIsStopped() *AgentRun {
	a.whens++
	a.runs = 1
	a.stopped = true
	return a
}

func (a *AgentRun) Then(description string) *AgentRun {
	return a
}

func (a *AgentRun) expect(check func() []spec.Problem) *AgentRun {
	a.checks = append(a.checks, check)
	return a
}

func (a *AgentRun) ExpectFailure() *AgentRun {
	a.expectFails = true
	return a
}

func (a *AgentRun) ExpectNoChange() *AgentRun {
	return a.expect(func() []spec.Problem {
		if a.change != nil {
			return agentProblems("expected no change, got %+v", *a.change)
		}
		return nil
	})
}

func (a *AgentRun) ExpectChange(base, title, description, file, content string) *AgentRun {
	return a.expect(func() []spec.Problem {
		if a.change == nil {
			return agentProblems("expected a change, got none")
		}
		var problems []spec.Problem
		if a.change.Base != base || a.change.Title != title || a.change.Description != description {
			problems = append(problems, agentProblems("expected a change onto %q titled %q with description %q, got %+v", base, title, description, *a.change)...)
		}
		if got, err := a.remoteGit("show", a.change.Branch+":"+file); err != nil || got != content {
			problems = append(problems, agentProblems("expected branch %q to have %s with %q, got %q (%v)", a.change.Branch, file, content, got, err)...)
		}
		if _, err := a.remoteGit("show", base+":"+file); err == nil {
			problems = append(problems, agentProblems("expected %s not to have %s", base, file)...)
		}
		return problems
	})
}

func (a *AgentRun) ExpectBranches(count int, prefix string) *AgentRun {
	return a.expect(func() []spec.Problem {
		out, err := a.remoteGit("for-each-ref", "--format=%(refname:short)", "refs/heads")
		if err != nil {
			return agentProblems("cannot list branches: %v", err)
		}
		var pushed []string
		for branch := range strings.Lines(out) {
			if branch = strings.TrimSpace(branch); branch != "main" {
				pushed = append(pushed, branch)
			}
		}
		if len(pushed) != count {
			return agentProblems("expected %d pushed branches, got %v", count, pushed)
		}
		for _, branch := range pushed {
			if !strings.HasPrefix(branch, prefix) {
				return agentProblems("expected branch %q to start with %q", branch, prefix)
			}
		}
		return nil
	})
}

func (a *AgentRun) ExpectAgentNotStarted() *AgentRun {
	return a.expect(func() []spec.Problem {
		if _, err := os.Stat(filepath.Join(a.records, "args")); err == nil {
			return agentProblems("expected the agent not to start")
		}
		return nil
	})
}

func (a *AgentRun) ExpectAgentSaw(want AgentSaw) *AgentRun {
	return a.expect(func() []spec.Problem {
		var problems []spec.Problem
		if want.Commits != 0 && a.record("commits") != strconv.Itoa(want.Commits) {
			problems = append(problems, agentProblems("expected the agent to see %d commits, got %q", want.Commits, a.record("commits"))...)
		}
		if want.Branch != "" && a.record("branch") != want.Branch {
			problems = append(problems, agentProblems("expected the agent to see branch %q, got %q", want.Branch, a.record("branch"))...)
		}
		if args := a.agentArgs(); want.Prompt && (len(args) == 0 || args[len(args)-1] != agent.Prompt) {
			problems = append(problems, agentProblems("expected the prompt as the agent's last argument, got %q", args)...)
		}
		if want.InWorkingCopy && a.record("in-working-copy") != "yes" {
			problems = append(problems, agentProblems("expected the agent to start at the root of a clone of the repository")...)
		}
		if want.EmptyHome && a.record("home-entries") != "0" {
			problems = append(problems, agentProblems("expected CODEX_HOME to exist and be empty, it has %q entries", a.record("home-entries"))...)
		}
		if want.OwnTemporaryFolder {
			problems = append(problems, a.ownTemporaryFolderProblems()...)
		}
		return problems
	})
}

func (a *AgentRun) ownTemporaryFolderProblems() []spec.Problem {
	temporary := a.record("tmp")
	if a.record("tmp-exists") != "yes" {
		return agentProblems("expected TMPDIR to be an existing folder, got %q", temporary)
	}
	if temporary == a.record("cwd") || strings.HasPrefix(temporary, a.record("cwd")+"/") {
		return agentProblems("expected TMPDIR %q outside the working copy", temporary)
	}
	root := fmt.Sprintf("sandbox_workspace_write.writable_roots=[%q]", temporary)
	if !slices.Contains(a.agentArgs(), root) {
		return agentProblems("expected the agent's arguments to have %s, got %q", root, a.agentArgs())
	}
	return nil
}

func (a *AgentRun) ExpectGitAuthenticated(token string) *AgentRun {
	return a.expect(func() []spec.Problem {
		var problems []spec.Problem
		for _, service := range []string{"git-upload-pack", "git-receive-pack"} {
			if got := a.gitAuthenticated[service]; got != token {
				problems = append(problems, agentProblems("expected %s with token %q, got %q", service, token, got)...)
			}
		}
		return problems
	})
}

func (a *AgentRun) ExpectAgentArgs(want ...string) *AgentRun {
	return a.expect(func() []spec.Problem {
		rest := a.agentArgs()
		for _, arg := range want {
			i := slices.Index(rest, arg)
			if i < 0 {
				return agentProblems("expected the agent's arguments %q in this order, got %q", want, a.agentArgs())
			}
			rest = rest[i+1:]
		}
		return nil
	})
}

func (a *AgentRun) ExpectAgentArgsWithout(text string) *AgentRun {
	return a.expect(func() []spec.Problem {
		if slices.ContainsFunc(a.agentArgs(), func(arg string) bool { return strings.Contains(arg, text) }) {
			return agentProblems("expected the agent's arguments without %q, got %q", text, a.agentArgs())
		}
		return nil
	})
}

func (a *AgentRun) ExpectAgentEnv(name, value string) *AgentRun {
	return a.expect(func() []spec.Problem {
		if got := a.agentEnv(); !slices.Contains(got, name+"="+value) {
			return agentProblems("expected %s=%s in the agent's environment, got %q", name, value, got)
		}
		return nil
	})
}

func (a *AgentRun) ExpectAgentEnvWithout(texts ...string) *AgentRun {
	return a.expect(func() []spec.Problem {
		var problems []spec.Problem
		env := a.agentEnv()
		if len(env) == 0 {
			return agentProblems("the agent recorded no environment")
		}
		for _, text := range texts {
			if slices.ContainsFunc(env, func(pair string) bool { return strings.Contains(pair, text) }) {
				problems = append(problems, agentProblems("expected the agent's environment without %q", text)...)
			}
		}
		return problems
	})
}

func (a *AgentRun) ExpectWorkingCopyWithout(text string) *AgentRun {
	return a.expect(func() []spec.Problem {
		copied := filepath.Join(a.records, "copy")
		var problems []spec.Problem
		err := filepath.WalkDir(copied, func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return err
			}
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			lower := bytes.ToLower(content)
			if bytes.Contains(content, []byte(text)) || bytes.Contains(lower, []byte("extraheader")) {
				problems = append(problems, agentProblems("expected %s to hold no credentials", path)...)
			}
			return nil
		})
		if err != nil {
			return agentProblems("cannot read the agent's copy of the working copy: %v", err)
		}
		return problems
	})
}

func (a *AgentRun) ExpectAgentLeftNothing() *AgentRun {
	return a.expect(func() []spec.Problem {
		workingCopy, home, temporary := a.record("cwd"), a.record("home"), a.record("tmp")
		if workingCopy == "" || home == "" || temporary == "" {
			return agentProblems("the agent recorded no working copy, home, or temporary folder")
		}
		var problems []spec.Problem
		for _, path := range []string{workingCopy, home, temporary} {
			if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
				problems = append(problems, agentProblems("expected %s to be removed", path)...)
			}
		}
		for _, name := range []string{"pid", "child"} {
			if pid, err := strconv.Atoi(a.record(name)); err == nil && !processGone(pid) {
				problems = append(problems, agentProblems("expected the agent's process %d to be gone", pid)...)
			}
		}
		return problems
	})
}

func processGone(pid int) bool {
	deadline := time.Now().Add(processGoneWait)
	for time.Now().Before(deadline) {
		if errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}

func (a *AgentRun) Run() []spec.Problem {
	if a.whens != 1 {
		return agentProblems("expected one When call, got %d", a.whens)
	}
	root, err := os.MkdirTemp("", "gc-agent-spec-")
	if err != nil {
		return agentProblems("cannot create a folder: %v", err)
	}
	defer os.RemoveAll(root)
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return agentProblems("cannot resolve a folder: %v", err)
	}
	a.remote = filepath.Join(root, "git", agentRepository+".git")
	a.records = filepath.Join(root, "records")
	if err := os.MkdirAll(a.records, 0o700); err != nil {
		return agentProblems("cannot create a folder: %v", err)
	}
	if !a.noRepository {
		if err := a.createRemote(root); err != nil {
			return agentProblems("cannot create the repository: %v", err)
		}
	}
	binary := filepath.Join(root, "fake-agent")
	if err := os.WriteFile(binary, []byte(a.fakeAgentScript()), 0o700); err != nil {
		return agentProblems("cannot write the fake agent: %v", err)
	}
	defer a.killLeftovers()

	server, err := a.startGitHub(filepath.Join(root, "git"))
	if err != nil {
		return agentProblems("cannot start the fake GitHub: %v", err)
	}
	defer server.Close()

	codex := agent.Codex{
		Binary:  binary,
		Environ: a.environ(root),
		GitHub:  contract.GitHub{APIURL: server.URL, Token: a.env["GITHUB_TOKEN"]},
	}
	for range a.runs {
		if problems := a.work(codex); len(problems) > 0 {
			return problems
		}
	}

	var problems []spec.Problem
	if a.expectFails && a.err == nil {
		problems = append(problems, agentProblems("expected the run to fail, it returned %+v", a.change)...)
	}
	if !a.expectFails && a.err != nil {
		problems = append(problems, agentProblems("expected the run not to fail, got: %v", a.err)...)
	}
	for _, check := range a.checks {
		problems = append(problems, check()...)
	}
	return problems
}

func (a *AgentRun) work(codex agent.Codex) []spec.Problem {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		a.change, a.err = codex.Work(ctx, agentRepository)
	}()
	if a.stopped {
		if !a.waitHanging(finished) {
			return agentProblems("expected the agent to start and hang within %s", agentRunLimit)
		}
		cancel()
	}
	select {
	case <-finished:
		return nil
	case <-time.After(agentRunLimit):
		cancel()
		<-finished
		return agentProblems("expected the run to finish within %s", agentRunLimit)
	}
}

func (a *AgentRun) waitHanging(finished <-chan struct{}) bool {
	deadline := time.Now().Add(agentRunLimit)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(filepath.Join(a.records, "hanging")); err == nil {
			return true
		}
		select {
		case <-finished:
			return false
		case <-time.After(20 * time.Millisecond):
		}
	}
	return false
}

func (a *AgentRun) environ(root string) []string {
	env := map[string]string{"PATH": os.Getenv("PATH"), "HOME": root}
	maps.Copy(env, a.env)
	var result []string
	for _, name := range slices.Sorted(maps.Keys(env)) {
		if !slices.Contains(a.unset, name) {
			result = append(result, name+"="+env[name])
		}
	}
	return result
}

func (a *AgentRun) startGitHub(gitRoot string) (*httptest.Server, error) {
	gitPath, err := exec.Command("git", "--exec-path").Output()
	if err != nil {
		return nil, err
	}
	backend := &cgi.Handler{
		Path: filepath.Join(strings.TrimSpace(string(gitPath)), "git-http-backend"),
		Root: "/git",
		Env:  []string{"GIT_PROJECT_ROOT=" + gitRoot, "GIT_HTTP_EXPORT_ALL=1", "PATH=" + os.Getenv("PATH")},
	}
	token := a.env["GITHUB_TOKEN"]
	a.gitAuthenticated = map[string]string{}
	var authenticated sync.Mutex
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/"+agentRepository && r.Header.Get("Authorization") == "Bearer "+token:
			json.NewEncoder(w).Encode(map[string]string{"clone_url": server.URL + "/git/" + agentRepository + ".git"})
		case strings.HasPrefix(r.URL.Path, "/git/"):
			user, password, ok := r.BasicAuth()
			if !ok || user == "" || password != token {
				w.Header().Set("WWW-Authenticate", `Basic realm="git"`)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			if service := gitService(r); service != "" {
				authenticated.Lock()
				a.gitAuthenticated[service] = password
				authenticated.Unlock()
			}
			backend.ServeHTTP(w, r)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	return server, nil
}

func gitService(r *http.Request) string {
	if service := r.URL.Query().Get("service"); service != "" {
		return service
	}
	return path.Base(r.URL.Path)
}

func (a *AgentRun) createRemote(root string) error {
	seed := filepath.Join(root, "seed")
	for _, args := range [][]string{
		{"init", "--bare", "--initial-branch=main", a.remote},
		{"-C", a.remote, "config", "http.receivepack", "true"},
		{"init", "--initial-branch=main", seed},
	} {
		if _, err := specGit(args...); err != nil {
			return err
		}
	}
	for i := 1; i <= a.commits; i++ {
		file := fmt.Sprintf("commit-%d.txt", i)
		if err := os.WriteFile(filepath.Join(seed, file), []byte(file), 0o600); err != nil {
			return err
		}
		if _, err := specGit("-C", seed, "add", "."); err != nil {
			return err
		}
		if _, err := specGit("-C", seed, "commit", "--message", file); err != nil {
			return err
		}
	}
	_, err := specGit("-C", seed, "push", a.remote, "main")
	return err
}

func (a *AgentRun) remoteGit(args ...string) (string, error) {
	return specGit(append([]string{"-C", a.remote}, args...)...)
}

func specGit(args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-c", "user.name=spec", "-c", "user.email=spec@example.test"}, args...)...)
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null"}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %v: %w: %s", args, err, out)
	}
	return string(out), nil
}

func (a *AgentRun) fakeAgentScript() string {
	return `#!/bin/sh
records=` + shellQuote(a.records) + `
echo $$ > "$records/pid"
: > "$records/args"
for arg in "$@"; do printf '%s\0' "$arg" >> "$records/args"; done
answer=
schema=
while [ $# -gt 0 ]; do
  case "$1" in
    -o|--output-last-message) answer=$2; shift ;;
    --output-schema) schema=$2; shift ;;
  esac
  shift
done
[ -n "$answer" ] || exit 64
grep -q '"title"' "$schema" && grep -q '"description"' "$schema" || exit 65
env > "$records/env"
pwd -P > "$records/cwd"
printf '%s' "$CODEX_HOME" > "$records/home"
printf '%s' "$TMPDIR" > "$records/tmp"
if [ -d "$TMPDIR" ]; then echo yes > "$records/tmp-exists"; fi
if [ -d "$CODEX_HOME" ]; then ls -A "$CODEX_HOME" | wc -l | tr -d ' ' > "$records/home-entries"; fi
git rev-list --count HEAD > "$records/commits"
git rev-parse --abbrev-ref HEAD > "$records/branch"
if [ "$(git rev-parse --show-toplevel)" = "$(pwd -P)" ] && [ -f commit-1.txt ]; then echo yes > "$records/in-working-copy"; fi
cp -R . "$records/copy"
` + strings.Join(append(slices.Clone(a.script), a.finalScript...), "\n") + "\n"
}

func (a *AgentRun) record(name string) string {
	content, err := os.ReadFile(filepath.Join(a.records, name))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(content))
}

func (a *AgentRun) agentArgs() []string {
	content, err := os.ReadFile(filepath.Join(a.records, "args"))
	if err != nil {
		return nil
	}
	return strings.Split(strings.TrimSuffix(string(content), "\x00"), "\x00")
}

func (a *AgentRun) agentEnv() []string {
	return strings.Split(a.record("env"), "\n")
}

func (a *AgentRun) killLeftovers() {
	for _, name := range []string{"pid", "child"} {
		if pid, err := strconv.Atoi(a.record(name)); err == nil {
			syscall.Kill(pid, syscall.SIGKILL)
		}
	}
}

func shellQuote(text string) string {
	return "'" + strings.ReplaceAll(text, "'", `'\''`) + "'"
}

type agentProblem string

func (p agentProblem) String() string {
	return string(p)
}

func agentProblems(format string, args ...any) []spec.Problem {
	return []spec.Problem{agentProblem(fmt.Sprintf(format, args...))}
}
