package extensions

import (
	"io"
	"net/http"
	"reflect"
	"testing"
	"testing/synctest"
	"time"

	"github.com/Andrew-Sem/gc/framework/spec"
	"github.com/Andrew-Sem/gc/specs/contract"
)

func TestRunFindsGitHubAsked(t *testing.T) {
	s := newScenario(&spec.Scenario[contract.Fact]{})
	s.When(spec.Post("/repositories")).
		ExpectStatus(http.StatusOK).
		ExpectFacts().
		Expect(s.GitHubNotAsked())

	asksGitHub := func(deps contract.Dependencies) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			client := &http.Client{Transport: deps.GitHub.Transport}
			if resp, err := client.Get(deps.GitHub.APIURL + "/repos/acme/api"); err == nil {
				resp.Body.Close()
			}
			w.WriteHeader(http.StatusOK)
		})
	}

	problems := s.Run(s.app(asksGitHub))
	want := []spec.Problem{githubAsked{got: []string{"acme/api"}}}
	if !reflect.DeepEqual(problems, want) {
		t.Errorf("run returned %q, want %q", problems, want)
	}
}

func TestRunAcceptsGitHubNotAsked(t *testing.T) {
	s := newScenario(&spec.Scenario[contract.Fact]{})
	s.When(spec.Post("/repositories")).
		ExpectStatus(http.StatusOK).
		ExpectFacts().
		Expect(s.GitHubNotAsked())

	ignoresGitHub := func(contract.Dependencies) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
	}

	if problems := s.Run(s.app(ignoresGitHub)); len(problems) != 0 {
		t.Errorf("run returned %q, want no problems", problems)
	}
}

func TestFakeGitHubAnswersLikeGitHub(t *testing.T) {
	tests := []struct {
		name, path, auth string
		given            func(*fakeGitHub)
		want             int
	}{
		{"visible repository", "/repos/acme/api", "Bearer " + fakeGitHubToken, nil, http.StatusOK},
		{"invisible repository", "/repos/acme/web", "Bearer " + fakeGitHubToken, nil, http.StatusNotFound},
		{"no token", "/repos/acme/api", "", nil, http.StatusNotFound},
		{"wrong token", "/repos/acme/api", "Bearer wrong", nil, http.StatusUnauthorized},
		{"rejected token", "/repos/acme/api", "Bearer " + fakeGitHubToken, func(g *fakeGitHub) { g.rejectsToken = true }, http.StatusUnauthorized},
		{"canned answer", "/repos/acme/api", "Bearer " + fakeGitHubToken, func(g *fakeGitHub) { g.answer = &cannedAnswer{status: http.StatusServiceUnavailable} }, http.StatusServiceUnavailable},
		{"another endpoint", "/user/repos", "Bearer " + fakeGitHubToken, nil, http.StatusNotFound},
		{"no answer", "/repos/acme/api", "Bearer " + fakeGitHubToken, func(g *fakeGitHub) { g.doesNotAnswer = true }, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := newFakeGitHub()
			g.sees = []string{"acme/api"}
			if tt.given != nil {
				tt.given(g)
			}

			req, _ := http.NewRequest(http.MethodGet, fakeGitHubURL+tt.path, nil)
			if tt.auth != "" {
				req.Header.Set("Authorization", tt.auth)
			}
			resp, err := (&http.Client{Transport: g.transport()}).Do(req)
			got := 0
			if err == nil {
				got = resp.StatusCode
				resp.Body.Close()
			}
			if got != tt.want {
				t.Errorf("status = %d, want %d (err %v)", got, tt.want, err)
			}
		})
	}
}

func TestFakeGitHubEchoesToken(t *testing.T) {
	g := newFakeGitHub()
	g.echoesToken = true

	req, _ := http.NewRequest(http.MethodGet, fakeGitHubURL+"/repos/acme/api", nil)
	req.Header.Set("Authorization", "Bearer "+fakeGitHubToken)
	resp, err := (&http.Client{Transport: g.transport()}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	want := `{"message":"Not Found for Bearer ` + fakeGitHubToken + `"}`
	if string(body) != want {
		t.Errorf("body = %s, want %s", body, want)
	}
}

func TestFakeGitHubMeasuresHowLongItWasAwaited(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		g := newFakeGitHub()
		g.hangs = true

		client := &http.Client{Transport: g.transport(), Timeout: 5 * time.Second}
		if resp, err := client.Get(fakeGitHubURL + "/repos/acme/api"); err == nil {
			resp.Body.Close()
			t.Fatalf("got status %d, want no answer", resp.StatusCode)
		}
		if got := g.waitedSoFar(); got != 5*time.Second {
			t.Errorf("waited %s, want 5s", got)
		}
	})
}

func TestFakeGitHubServesOverNetwork(t *testing.T) {
	g := newFakeGitHub()
	url := g.listen()
	defer g.server.Close()
	g.sees = []string{"acme/api"}

	req, _ := http.NewRequest(http.MethodGet, url+"/repos/acme/api", nil)
	req.Header.Set("Authorization", "Bearer "+fakeGitHubToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestGitHubAskedString(t *testing.T) {
	p := githubAsked{got: []string{"acme/api"}}
	want := "expected GitHub not to be asked, got [acme/api]"
	if got := p.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}
