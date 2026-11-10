package extensions

import (
	"context"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/Andrew-Sem/gc/framework/journal"
	"github.com/Andrew-Sem/gc/framework/spec"
	"github.com/Andrew-Sem/gc/internal/domain"
	"github.com/Andrew-Sem/gc/internal/web"
	"github.com/Andrew-Sem/gc/specs/contract"
)

type Feature struct {
	t      *testing.T
	core   *spec.Feature[contract.Fact]
	binary string
}

func NewFeature(t *testing.T, path string) *Feature {
	return &Feature{t: t, core: spec.NewFeature[contract.Fact](t, path)}
}

func (f *Feature) Run(name string, describe func(*Scenario)) {
	f.t.Helper()
	var scenario *Scenario
	f.core.Run(name, func(core *spec.Scenario[contract.Fact]) {
		scenario = newScenario(core)
		describe(scenario)
	}, func(j *journal.Journal[contract.Fact]) http.Handler {
		return scenario.app(web.New)(j)
	})
}

func (s *Scenario) WhenWokenUp() *Scenario {
	s.WhenAction(func(j *journal.Journal[contract.Fact]) { domain.WakeUp(j, time.Now(), s.baseURL) })
	return s
}

func (s *Scenario) WhenWokenUpFor(wait time.Duration) *Scenario {
	s.WhenAction(func(j *journal.Journal[contract.Fact]) {
		ctx, cancel := context.WithCancel(context.Background())
		var runs sync.WaitGroup
		web.WakeUp(ctx, s.dependencies(j), &runs)
		time.Sleep(wait)
		cancel()
		runs.Wait()
	})
	return s
}

func (s *Scenario) WhenRunning(start string, runFor time.Duration) *Scenario {
	s.WhenAction(func(j *journal.Journal[contract.Fact]) {
		time.Sleep(untilClock(start))
		ctx, cancel := context.WithCancel(context.Background())
		stopped := make(chan struct{})
		go func() {
			defer close(stopped)
			web.Serve(ctx, newIdleListener(), s.dependencies(j))
		}()
		for _, r := range s.requests {
			var request contract.Fact = contract.RunRequested{Repository: r.repository}
			if r.sweep {
				request = contract.SweepRequested{Repository: r.repository}
			}
			if r.fact != nil {
				request = r.fact
			}
			time.AfterFunc(untilClock(r.clock), func() { j.Append(request) })
		}
		time.Sleep(runFor)
		cancel()
		<-stopped
	})
	return s
}

func untilClock(clock string) time.Duration {
	at, err := time.Parse("15:04:05", clock)
	if err != nil {
		at, err = time.Parse("15:04", clock)
	}
	if err != nil {
		panic(err)
	}
	now := time.Now()
	return now.Truncate(24 * time.Hour).Add(at.Sub(at.Truncate(24 * time.Hour))).Sub(now)
}

type idleListener struct {
	closed chan struct{}
}

func newIdleListener() *idleListener {
	return &idleListener{closed: make(chan struct{})}
}

func (l *idleListener) Accept() (net.Conn, error) {
	<-l.closed
	return nil, net.ErrClosed
}

func (l *idleListener) Close() error {
	select {
	case <-l.closed:
	default:
		close(l.closed)
	}
	return nil
}

func (l *idleListener) Addr() net.Addr {
	return &net.TCPAddr{}
}

func (s *Scenario) app(newApp func(contract.Dependencies) http.Handler) func(*journal.Journal[contract.Fact]) http.Handler {
	return func(j *journal.Journal[contract.Fact]) http.Handler {
		app := newApp(s.dependencies(j))
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			asOperator(r)
			app.ServeHTTP(w, r)
		})
	}
}

func (s *Scenario) dependencies(j *journal.Journal[contract.Fact]) contract.Dependencies {
	return contract.Dependencies{
		Journal:       j,
		GitHub:        contract.GitHub{APIURL: fakeGitHubURL, AppID: fakeAppID, PrivateKey: fakeAppKey(), Transport: s.github.transport()},
		Agent:         s.agent,
		WebhookSecret: fakeWebhookSecret,
		Admin:         s.admin,
		OpenAIBaseURL: s.baseURL,
	}
}
