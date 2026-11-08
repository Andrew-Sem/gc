package extensions

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Andrew-Sem/gc/framework/journal"
	"github.com/Andrew-Sem/gc/framework/spec"
	"github.com/Andrew-Sem/gc/internal/agent"
	"github.com/Andrew-Sem/gc/internal/web"
	"github.com/Andrew-Sem/gc/specs/contract"
)

const (
	QuotedInStatus   = "HTTP/1.1 %s oops\r\n\r\n"
	QuotedInRedirect = "HTTP/1.1 301 Moved\r\nLocation: http://[bad/%s\r\nContent-Length: 0\r\n\r\n"

	wakeUpLimit = 30 * time.Second
)

type WakeUpRun struct {
	covers    []string
	github    *fakeGitHub
	agent     contract.Agent
	realAgent bool
	whens     int
	outcome   contract.Outcome
	logged    []string
	notLogged []string
}

type reasonAgent struct {
	change *contract.AgentChange
	err    error
}

func (a reasonAgent) Work(context.Context, string) (contract.AgentResult, error) {
	return contract.AgentResult{Change: a.change}, a.err
}

func (f *Feature) RunWakeUp(name string, describe func(*WakeUpRun)) {
	f.t.Helper()
	f.core.RunCustom(name, func() spec.Custom {
		w := &WakeUpRun{github: newFakeGitHub()}
		describe(w)
		return w
	})
}

func (w *WakeUpRun) Covers(ids ...string) *WakeUpRun {
	w.covers = ids
	return w
}

func (w *WakeUpRun) Covered() []string {
	return w.covers
}

func (w *WakeUpRun) GivenAgentFailsWith(reason string) *WakeUpRun {
	w.agent = reasonAgent{err: errors.New(reason)}
	return w
}

func (w *WakeUpRun) GivenAgentChanged() *WakeUpRun {
	w.agent = reasonAgent{change: &contract.AgentChange{Branch: "gc/run-1", Base: "main", Title: "Remove dead code", Description: "The helper has no callers."}}
	return w
}

func (w *WakeUpRun) GivenTheRealAgentAsksForTheCloneURL() *WakeUpRun {
	w.realAgent = true
	return w
}

func (w *WakeUpRun) GivenGitHubAnswers(status int) *WakeUpRun {
	w.github.given(func(g *fakeGitHub) { g.answer = &cannedAnswer{status: status} })
	return w
}

func (w *WakeUpRun) GivenGitHubQuotesToken(form string) *WakeUpRun {
	w.github.given(func(g *fakeGitHub) { g.quotesToken = form })
	return w
}

func (w *WakeUpRun) WhenWokenUp() *WakeUpRun {
	w.whens++
	return w
}

func (w *WakeUpRun) Then(description string) *WakeUpRun {
	return w
}

func (w *WakeUpRun) ExpectOutcome(outcome contract.Outcome) *WakeUpRun {
	w.outcome = outcome
	return w
}

func (w *WakeUpRun) ExpectLog(texts ...string) *WakeUpRun {
	w.logged = append(w.logged, texts...)
	return w
}

func (w *WakeUpRun) ExpectLogWithout(texts ...string) *WakeUpRun {
	w.notLogged = append(w.notLogged, texts...)
	return w
}

func (w *WakeUpRun) GitHubToken() string {
	return fakeGitHubToken
}

func (w *WakeUpRun) Run() []spec.Problem {
	if w.whens != 1 {
		return []spec.Problem{launchProblem("expected one WhenWokenUp call")}
	}
	api := w.github.listen()
	defer w.github.server.Close()
	github := contract.GitHub{APIURL: api, AppID: fakeAppID, PrivateKey: fakeAppKey()}
	dir, err := os.MkdirTemp("", "gc-wake-up-")
	if err != nil {
		return []spec.Problem{launchProblem(fmt.Sprintf("cannot create a folder: %v", err))}
	}
	defer os.RemoveAll(dir)
	worker := w.agent
	if w.realAgent {
		worker = agent.Codex{Binary: "/nonexistent/codex", Environ: []string{"PATH=" + os.Getenv("PATH"), "HOME=" + dir, "TMPDIR=" + dir}, GitHub: github}
	}
	j := &journal.Journal[contract.Fact]{}
	j.Append(contract.RepositoryConnected{FullName: agentRepository})
	deps := contract.Dependencies{Journal: j, GitHub: github, Agent: worker, WebhookSecret: fakeWebhookSecret}

	output, finished := w.wake(deps, dir)
	var problems []spec.Problem
	if !finished {
		problems = append(problems, launchProblem(fmt.Sprintf("expected the run to end within %s", wakeUpLimit)))
	}
	if w.outcome != "" {
		problems = append(problems, w.outcomeProblems(j.Facts())...)
	}
	for _, text := range w.logged {
		if !strings.Contains(output, text) {
			problems = append(problems, launchProblem(fmt.Sprintf("expected the log to contain %q, got %q", text, output)))
		}
	}
	for _, text := range w.notLogged {
		if strings.Contains(output, text) {
			problems = append(problems, launchProblem(fmt.Sprintf("expected the log not to contain %q, got %q", text, output)))
		}
	}
	return problems
}

func (w *WakeUpRun) wake(deps contract.Dependencies, dir string) (string, bool) {
	file, err := os.CreateTemp(dir, "output")
	if err != nil {
		panic(err)
	}
	defer file.Close()
	stderr, stdout, writer := os.Stderr, os.Stdout, log.Writer()
	os.Stderr, os.Stdout = file, file
	log.SetOutput(file)
	defer func() {
		os.Stderr, os.Stdout = stderr, stdout
		log.SetOutput(writer)
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var runs sync.WaitGroup
	web.WakeUp(ctx, deps, &runs)
	done := make(chan struct{})
	go func() {
		runs.Wait()
		close(done)
	}()
	finished := true
	select {
	case <-done:
	case <-time.After(wakeUpLimit):
		finished = false
	}
	content, err := os.ReadFile(file.Name())
	if err != nil {
		panic(err)
	}
	return string(content), finished
}

func (w *WakeUpRun) outcomeProblems(facts []contract.Fact) []spec.Problem {
	i := slices.IndexFunc(facts, func(f contract.Fact) bool {
		_, ok := f.(contract.AgentRunFinished)
		return ok
	})
	if i < 0 {
		return []spec.Problem{launchProblem("expected AgentRunFinished, got none")}
	}
	if got := facts[i].(contract.AgentRunFinished).Outcome; got != w.outcome {
		return []spec.Problem{launchProblem(fmt.Sprintf("expected the outcome %q, got %q", w.outcome, got))}
	}
	return nil
}
