package extensions

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"net"
	"net/http"
	"net/http/cgi"
	"net/http/httptest"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
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

	reportLinkWindow   = 30
	recordsPlaceholder = "RECORDS_FOLDER"
)

var answerFields = []string{
	"change", "description", "evidence", "finding", "follow_up", "rejected_candidates",
	"risk", "rollback", "selected_candidate", "slug", "title", "validation",
}

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

	token         string
	tokensExpire  bool
	appUnknown    bool
	openPRs       []fakeOpenPR
	prsUnlisted   bool
	prsListedIn   time.Duration
	tokensIssued  []string
	tokenRequests []string
	tokensMu      sync.Mutex

	seedFiles   map[string]string
	gitlink     bool
	cloneStalls bool
	stopAfter   time.Duration
	panicked    any
	otherRemote bool

	cloneCredentials string

	gitServer *httptest.Server
	cleanups  []func()

	remote      string
	records     string
	change      *contract.AgentChange
	explanation string
	tokens      contract.TokenUsage
	baseURL     string
	err         error
}

func (f *Feature) RunAgent(name string, describe func(*AgentRun)) {
	f.t.Helper()
	f.core.RunCustom(name, func() spec.Custom {
		a := &AgentRun{env: map[string]string{
			"GITHUB_APP_ID":               fakeAppID,
			"GITHUB_APP_PRIVATE_KEY_FILE": "/operator/gc-app.pem",
			"GC_WEBHOOK_SECRET":           fakeWebhookSecret,
			"OPENAI_API_KEY":              fakeOpenAIKey,
		}, token: fakeGitHubToken}
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

func (a *AgentRun) GivenTaskBaseURL(url string) *AgentRun {
	a.baseURL = url
	return a
}

func (a *AgentRun) GivenInstallationToken(token string) *AgentRun {
	a.token = token
	return a
}

type fakeOpenPR struct {
	Number int
	Title  string
	Branch string
	Files  []string
}

func (a *AgentRun) GivenOpenPR(title, branch string, files ...string) *AgentRun {
	a.openPRs = append(a.openPRs, fakeOpenPR{Number: len(a.openPRs) + 1, Title: title, Branch: branch, Files: files})
	return a
}

func (a *AgentRun) GivenGitHubListsPRsAfter(wait time.Duration) *AgentRun {
	a.prsListedIn = wait
	return a
}

func (a *AgentRun) GivenGitHubCannotListPRs() *AgentRun {
	a.prsUnlisted = true
	return a
}

func (a *AgentRun) ExpectPromptWithout(texts ...string) *AgentRun {
	return a.expect(func() []spec.Problem {
		args := a.agentArgs()
		var problems []spec.Problem
		for _, text := range texts {
			if len(args) > 0 && strings.Contains(args[len(args)-1], text) {
				problems = append(problems, agentProblems("expected the prompt not to say %q", text)...)
			}
		}
		return problems
	})
}

func (a *AgentRun) servePulls(w http.ResponseWriter, path string) {
	if a.prsUnlisted {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	time.Sleep(a.prsListedIn)
	pulls := "/repos/" + agentRepository + "/pulls"
	if path == pulls {
		var list []map[string]any
		for _, pr := range a.openPRs {
			list = append(list, map[string]any{"number": pr.Number, "title": pr.Title, "head": map[string]string{"ref": pr.Branch}})
		}
		json.NewEncoder(w).Encode(append([]map[string]any{}, list...))
		return
	}
	for _, pr := range a.openPRs {
		if path == fmt.Sprintf("%s/%d/files", pulls, pr.Number) {
			var files []map[string]string
			for _, file := range pr.Files {
				files = append(files, map[string]string{"filename": file})
			}
			json.NewEncoder(w).Encode(append([]map[string]string{}, files...))
			return
		}
	}
	w.WriteHeader(http.StatusNotFound)
}

func (a *AgentRun) GivenGitHubCannotNameTheApp() *AgentRun {
	a.appUnknown = true
	return a
}

func (a *AgentRun) GivenTokensExpireWhileTheAgentWorks() *AgentRun {
	a.tokensExpire = true
	a.script = append(a.script, `touch "RECORDS_FOLDER/tokens-expired"`)
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

func (a *AgentRun) GivenRepositoryFile(file, content string) *AgentRun {
	if a.seedFiles == nil {
		a.seedFiles = map[string]string{}
	}
	a.seedFiles[file] = content
	return a
}

func (a *AgentRun) GivenRepositoryWithGitlink() *AgentRun {
	a.gitlink = true
	return a
}

func (a *AgentRun) GivenCloneStalls() *AgentRun {
	a.cloneStalls = true
	return a
}

func (a *AgentRun) GivenAgentChanges(file, content string) *AgentRun {
	a.script = append(a.script, fmt.Sprintf(`mkdir -p "$(dirname %[1]s)"; printf '%%s' %[2]s > %[1]s`, shellQuote(file), shellQuote(content)))
	return a
}

func (a *AgentRun) GivenAgentRuns(script string) *AgentRun {
	a.script = append(a.script, script)
	return a
}

func (a *AgentRun) CommandPath() string {
	return a.prepared("command.sh")
}

func (a *AgentRun) OutsidePath() string {
	return a.prepared("outside")
}

func (a *AgentRun) prepared(name string) string {
	return recordsPlaceholder + "/" + name
}

func (a *AgentRun) GivenAnotherRepositoryOnTheRemote() *AgentRun {
	a.otherRemote = true
	return a
}

func (a *AgentRun) GivenAgentLinksReportPathsTo(target string) *AgentRun {
	return a.occupyReportPaths(func(path string) string {
		return fmt.Sprintf("ln -s %s %s", shellQuote(target), path)
	})
}

func (a *AgentRun) GivenAgentWritesToReportPaths() *AgentRun {
	return a.occupyReportPaths(func(path string) string {
		return "echo mine > " + path
	})
}

func (a *AgentRun) occupyReportPaths(occupy func(path string) string) *AgentRun {
	lines := []string{"mkdir -p docs/gc-reviews"}
	for i := range reportLinkWindow {
		name := time.Now().UTC().Add(time.Duration(i) * time.Second).Format("20060102-150405Z")
		lines = append(lines, occupy("docs/gc-reviews/"+name+"-dead-code.md"))
	}
	a.script = append(a.script, strings.Join(lines, "\n"))
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

func (a *AgentRun) GivenAgentFailsWith(reason string) *AgentRun {
	a.finalScript = append(a.finalScript, fmt.Sprintf(`printf %s >&2; exit 3`, shellQuote(reason)))
	return a
}

func (a *AgentRun) GivenCloneURLHolds(credentials string) *AgentRun {
	a.cloneCredentials = credentials
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

func (a *AgentRun) WhenRunIsStoppedAfter(wait time.Duration) *AgentRun {
	a.whens++
	a.runs = 1
	a.stopAfter = wait
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

func (a *AgentRun) ExpectFailureReason(text string) *AgentRun {
	return a.expect(func() []spec.Problem {
		if a.err == nil || !strings.Contains(a.err.Error(), text) {
			return agentProblems("expected the failure to give the reason %q, got: %v", text, a.err)
		}
		return nil
	})
}

func (a *AgentRun) ExpectFailureReasonWithout(texts ...string) *AgentRun {
	return a.expect(func() []spec.Problem {
		var problems []spec.Problem
		for _, text := range texts {
			if a.err != nil && strings.Contains(a.err.Error(), text) {
				problems = append(problems, agentProblems("expected the failure not to mention %q, got: %v", text, a.err)...)
			}
		}
		return problems
	})
}

func (a *AgentRun) ExpectNoChange() *AgentRun {
	return a.expect(func() []spec.Problem {
		if a.change != nil {
			return agentProblems("expected no change, got %+v", *a.change)
		}
		return nil
	})
}

func (a *AgentRun) GivenAgentPrints(lines ...string) *AgentRun {
	for _, line := range lines {
		a.script = append(a.script, fmt.Sprintf(`printf '%%s\n' %s`, shellQuote(line)))
	}
	return a
}

func (a *AgentRun) GivenAgentPrintsLast(line string) *AgentRun {
	a.finalScript = append(a.finalScript, fmt.Sprintf(`printf '%%s' %s`, shellQuote(line)))
	return a
}

func (a *AgentRun) ExpectTokens(want contract.TokenUsage) *AgentRun {
	return a.expect(func() []spec.Problem {
		if a.tokens != want {
			return agentProblems("expected the tokens %+v, got %+v", want, a.tokens)
		}
		return nil
	})
}

func (a *AgentRun) ExpectExplanation(text string) *AgentRun {
	return a.expect(func() []spec.Problem {
		if a.explanation != text {
			return agentProblems("expected the explanation %q, got %q", text, a.explanation)
		}
		return nil
	})
}

func (a *AgentRun) ExpectChange(base, title, file, content string) *AgentRun {
	return a.expect(func() []spec.Problem {
		if a.change == nil {
			return agentProblems("expected a change, got none")
		}
		var problems []spec.Problem
		if a.change.Base != base || a.change.Title != title {
			problems = append(problems, agentProblems("expected a change onto %q titled %q, got %+v", base, title, *a.change)...)
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

func (a *AgentRun) ExpectReport(slug, content string) *AgentRun {
	return a.expect(func() []spec.Problem {
		path, problems := a.reportPath(slug)
		if len(problems) > 0 {
			return problems
		}
		if got, err := a.remoteGit("show", a.change.Branch+":"+path); err != nil || got != content {
			return agentProblems("expected the report to read %q, got %q (%v)", content, got, err)
		}
		if commits, err := a.remoteGit("rev-list", "--count", "main.."+a.change.Branch); err != nil || strings.TrimSpace(commits) != "1" {
			return agentProblems("expected the report in the agent's one commit, got %q commits (%v)", commits, err)
		}
		return nil
	})
}

func (a *AgentRun) ExpectDescriptionEndingWithReportPath(prefix string) *AgentRun {
	return a.expect(func() []spec.Problem {
		path, problems := a.reportPath("dead-code")
		if len(problems) > 0 {
			return problems
		}
		if a.change.Description != prefix+path {
			return agentProblems("expected description %q, got %q", prefix+path, a.change.Description)
		}
		return nil
	})
}

func (a *AgentRun) reportPath(slug string) (string, []spec.Problem) {
	if a.change == nil {
		return "", agentProblems("expected a change, got none")
	}
	out, err := a.remoteGit("diff", "--name-only", "main", a.change.Branch, "--", "docs/gc-reviews")
	if err != nil {
		return "", agentProblems("cannot list the reports: %v", err)
	}
	path := strings.TrimSpace(out)
	if !regexp.MustCompile(`^docs/gc-reviews/[0-9]{8}-[0-9]{6}Z-` + regexp.QuoteMeta(slug) + `\.md$`).MatchString(path) {
		return "", agentProblems("expected one report for %q, got %q", slug, out)
	}
	return path, nil
}

func (a *AgentRun) ExpectCommittedBy(identity string) *AgentRun {
	return a.expect(func() []spec.Problem {
		out, err := a.remoteGit("log", "-1", "--format=%an <%ae>%n%cn <%ce>", "--branches=gc/*")
		if err != nil {
			return agentProblems("cannot read the pushed commit: %v", err)
		}
		if want := identity + "\n" + identity; strings.TrimSpace(out) != want {
			return agentProblems("expected the commit's author and committer to be %s, got %q", identity, out)
		}
		return nil
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

func (a *AgentRun) ExpectNoCommandFromTheAgentRan() *AgentRun {
	return a.expect(func() []spec.Problem {
		if a.record("command-ran") != "" {
			return agentProblems("a command chosen by the agent ran outside its sandbox: %s", a.record("command-ran"))
		}
		return nil
	})
}

func (a *AgentRun) ExpectNothingPushedToOtherRepository() *AgentRun {
	return a.expect(func() []spec.Problem {
		out, err := specGit("-C", a.otherRemotePath(), "for-each-ref", "refs/heads")
		if err != nil {
			return agentProblems("cannot read the other repository: %v", err)
		}
		if strings.TrimSpace(out) != "" {
			return agentProblems("expected nothing in the other repository, got %q", out)
		}
		return nil
	})
}

func (a *AgentRun) ExpectFolderEmpty(name string) *AgentRun {
	return a.expect(func() []spec.Problem {
		entries, err := os.ReadDir(filepath.Join(a.records, name))
		if err != nil {
			return agentProblems("cannot read %s: %v", name, err)
		}
		if len(entries) != 0 {
			return agentProblems("expected nothing written to %s, got %v", name, entries)
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

func (a *AgentRun) ExpectTokensLimitedToTheRepository() *AgentRun {
	return a.expect(func() []spec.Problem {
		a.tokensMu.Lock()
		defer a.tokensMu.Unlock()
		if len(a.tokenRequests) == 0 {
			return agentProblems("expected a token request, got none")
		}
		var problems []spec.Problem
		for _, body := range a.tokenRequests {
			var request struct {
				Repositories []string `json:"repositories"`
			}
			if json.Unmarshal([]byte(body), &request) != nil || !slices.Equal(request.Repositories, []string{"api"}) {
				problems = append(problems, agentProblems("expected a token for the repository api alone, asked with %s", body)...)
			}
		}
		return problems
	})
}

func (a *AgentRun) issueToken(body string) string {
	a.tokensMu.Lock()
	defer a.tokensMu.Unlock()
	a.tokenRequests = append(a.tokenRequests, body)
	token := a.token
	if a.tokensExpire {
		token = fmt.Sprintf("%s-%d", a.token, len(a.tokensIssued)+1)
	}
	a.tokensIssued = append(a.tokensIssued, token)
	return token
}

func (a *AgentRun) tokenValid(token string) bool {
	a.tokensMu.Lock()
	defer a.tokensMu.Unlock()
	if !slices.Contains(a.tokensIssued, token) {
		return false
	}
	if _, err := os.Stat(filepath.Join(a.records, "tokens-expired")); a.tokensExpire && err == nil {
		return token == a.tokensIssued[len(a.tokensIssued)-1] && len(a.tokensIssued) > 1
	}
	return true
}

func (a *AgentRun) ExpectPromptWith(texts ...string) *AgentRun {
	return a.expect(func() []spec.Problem {
		args := a.agentArgs()
		if len(args) == 0 {
			return agentProblems("the agent recorded no arguments")
		}
		var problems []spec.Problem
		for _, text := range texts {
			if !strings.Contains(args[len(args)-1], text) {
				problems = append(problems, agentProblems("expected the prompt to say %q", text)...)
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

func (a *AgentRun) openLeftovers() {
	for _, name := range []string{"cwd", "home", "tmp"} {
		folder := a.record(name)
		if folder == "" {
			continue
		}
		filepath.WalkDir(folder, func(path string, entry fs.DirEntry, _ error) error {
			if entry != nil && entry.IsDir() {
				os.Chmod(path, 0o700)
			}
			return nil
		})
		os.RemoveAll(folder)
	}
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
	if err := a.prepareRecords(root); err != nil {
		return agentProblems("cannot prepare the records: %v", err)
	}
	binary := filepath.Join(root, "fake-agent")
	if err := os.WriteFile(binary, []byte(a.fakeAgentScript()), 0o700); err != nil {
		return agentProblems("cannot write the fake agent: %v", err)
	}
	defer a.openLeftovers()
	defer a.killLeftovers()
	defer func() {
		for _, cleanup := range a.cleanups {
			cleanup()
		}
	}()

	server, err := a.startGitHub(filepath.Join(root, "git"))
	if err != nil {
		return agentProblems("cannot start the fake GitHub: %v", err)
	}
	defer server.Close()

	codex := agent.Codex{
		Binary:  binary,
		Environ: a.environ(root),
		GitHub:  contract.GitHub{APIURL: server.URL, AppID: fakeAppID, PrivateKey: fakeAppKey()},
	}
	for range a.runs {
		if problems := a.work(codex); len(problems) > 0 {
			return problems
		}
	}

	problems := a.schemaProblems()
	if a.panicked != nil {
		problems = append(problems, agentProblems("the run panicked: %v", a.panicked)...)
	}
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

func (a *AgentRun) schemaProblems() []spec.Problem {
	raw, err := os.ReadFile(filepath.Join(a.records, "schema"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return agentProblems("cannot read the answer schema: %v", err)
	}
	var schema struct {
		Properties           map[string]struct{ Type string } `json:"properties"`
		Required             []string                         `json:"required"`
		AdditionalProperties *bool                            `json:"additionalProperties"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		return agentProblems("the answer schema is not JSON: %v", err)
	}
	properties := slices.Sorted(maps.Keys(schema.Properties))
	required := slices.Sorted(slices.Values(schema.Required))
	if !slices.Equal(properties, answerFields) || !slices.Equal(required, answerFields) || schema.AdditionalProperties == nil || *schema.AdditionalProperties {
		return agentProblems("expected the answer schema to require exactly %q, got %s", answerFields, raw)
	}
	return nil
}

func (a *AgentRun) work(codex agent.Codex) []spec.Problem {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		defer func() {
			if failure := recover(); failure != nil {
				a.panicked = failure
				a.err = errors.New("panic")
			}
		}()
		var result contract.AgentResult
		result, a.err = codex.Work(ctx, contract.AgentTask{Repository: agentRepository, OpenAIBaseURL: a.baseURL})
		a.change, a.explanation, a.tokens = result.Change, result.Explanation, result.Tokens
	}()
	if a.stopAfter != 0 {
		time.Sleep(a.stopAfter)
		cancel()
	}
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
		select {
		case <-finished:
		case <-time.After(processGoneWait):
		}
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

func (a *AgentRun) cloneURL(server string) string {
	address := server + "/git/" + agentRepository + ".git"
	if a.cloneCredentials != "" {
		address = strings.Replace(address, "://", "://"+a.cloneCredentials+"@", 1)
	}
	return address
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
	a.gitAuthenticated = map[string]string{}
	var authenticated sync.Mutex
	var server *httptest.Server
	silent, err := a.silentAddress()
	if err != nil {
		return nil, err
	}
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, _ := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		switch {
		case strings.HasPrefix(r.URL.Path, "/repos/"+agentRepository+"/pulls") && r.Method == http.MethodGet && a.tokenValid(token):
			a.servePulls(w, r.URL.Path)
		case r.URL.Path == "/app" && a.appUnknown:
			w.WriteHeader(http.StatusInternalServerError)
		case r.URL.Path == "/app" && validAppToken(r.Header.Get("Authorization")):
			fmt.Fprint(w, `{"slug":"gc-spec"}`)
		case r.URL.Path == "/users/gc-spec[bot]" && a.tokenValid(token):
			fmt.Fprint(w, `{"id":4711,"login":"gc-spec[bot]","type":"Bot"}`)
		case r.URL.Path == "/repos/"+agentRepository+"/installation" && validAppToken(r.Header.Get("Authorization")):
			fmt.Fprint(w, `{"id":7}`)
		case r.URL.Path == fakeInstallationPath && r.Method == http.MethodPost && validAppToken(r.Header.Get("Authorization")):
			body, _ := io.ReadAll(r.Body)
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]string{"token": a.issueToken(string(body))})
		case r.URL.Path == "/repos/"+agentRepository && a.tokenValid(token) && a.cloneStalls:
			json.NewEncoder(w).Encode(map[string]string{"clone_url": "http://" + silent + "/x.git"})
		case r.URL.Path == "/repos/"+agentRepository && a.tokenValid(token):
			json.NewEncoder(w).Encode(map[string]string{"clone_url": a.cloneURL(server.URL)})
		case strings.HasPrefix(r.URL.Path, "/git/"):
			user, password, ok := r.BasicAuth()
			if !ok || user == "" || !a.tokenValid(password) {
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
	a.gitServer = server
	return server, nil
}

func (a *AgentRun) silentAddress() (string, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	a.cleanups = append(a.cleanups, func() { listener.Close() })
	var conns sync.Map
	a.cleanups = append(a.cleanups, func() {
		conns.Range(func(conn, _ any) bool { conn.(net.Conn).Close(); return true })
	})
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			conns.Store(conn, true)
		}
	}()
	return listener.Addr().String(), nil
}

func (a *AgentRun) otherRemotePath() string {
	return filepath.Join(filepath.Dir(a.remote), "other.git")
}

func (a *AgentRun) prepareRecords(root string) error {
	if err := os.Mkdir(filepath.Join(a.records, "outside"), 0o700); err != nil {
		return err
	}
	command := fmt.Sprintf("#!/bin/sh\necho \"$0 $*\" >> %s\nexit 1\n", shellQuote(filepath.Join(a.records, "command-ran")))
	if err := os.WriteFile(filepath.Join(a.records, "command.sh"), []byte(command), 0o700); err != nil {
		return err
	}
	if a.otherRemote {
		if _, err := specGit("init", "--bare", "--initial-branch=main", a.otherRemotePath()); err != nil {
			return err
		}
		for _, setting := range []string{"http.receivepack", "receive.shallowUpdate"} {
			if _, err := specGit("-C", a.otherRemotePath(), "config", setting, "true"); err != nil {
				return err
			}
		}
	}
	return nil
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
	for file, content := range a.seedFiles {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(seed, file)), 0o700); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(seed, file), []byte(content), 0o600); err != nil {
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
	if a.gitlink {
		head, err := specGit("-C", seed, "rev-parse", "HEAD")
		if err != nil {
			return err
		}
		if _, err := specGit("-C", seed, "update-index", "--add", "--cacheinfo", "160000,"+strings.TrimSpace(head)+",sub"); err != nil {
			return err
		}
		if _, err := specGit("-C", seed, "commit", "--message", "gitlink"); err != nil {
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
cp "$schema" "$records/schema" || exit 65
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
` + strings.ReplaceAll(strings.Join(append(slices.Clone(a.script), a.finalScript...), "\n"), recordsPlaceholder, a.records) + "\n"
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
