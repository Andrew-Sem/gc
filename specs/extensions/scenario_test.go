package extensions

import (
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
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

func TestFakeGitHubAnswersInItsOwnCasing(t *testing.T) {
	g := newFakeGitHub()
	g.sees = []string{"acme/api"}
	req, _ := http.NewRequest(http.MethodGet, fakeGitHubURL+"/repos/acme/api", nil)
	req.Header.Set("Authorization", "Bearer "+fakeGitHubToken)
	resp, err := (&http.Client{Transport: g.transport()}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if want := `{"full_name":"Acme/Api"}`; string(body) != want {
		t.Errorf("body = %s, want %s", body, want)
	}
}

func TestFakeGitHubMovesRepository(t *testing.T) {
	g := newFakeGitHub()
	g.moves = http.StatusMovedPermanently
	ask := func(client *http.Client) (int, string) {
		req, _ := http.NewRequest(http.MethodGet, fakeGitHubURL+"/repos/acme/api", nil)
		req.Header.Set("Authorization", "Bearer "+fakeGitHubToken)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, resp.Header.Get("Location") + string(body)
	}

	noFollow := &http.Client{Transport: g.transport(), CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	if status, got := ask(noFollow); status != http.StatusMovedPermanently || !strings.HasPrefix(got, movedRepositoryPath) {
		t.Errorf("without following: %d %s, want 301 to %s", status, got, movedRepositoryPath)
	}
	if status, got := ask(&http.Client{Transport: g.transport()}); status != http.StatusOK || got != `{"full_name":"Acme/Api-Renamed"}` {
		t.Errorf("following: %d %s, want 200 with the renamed repository", status, got)
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
	tests := []struct {
		problem githubAsked
		want    string
	}{
		{githubAsked{got: []string{"acme/api"}}, "expected GitHub not to be asked, got [acme/api]"},
		{githubAsked{want: []string{"acme/api"}, got: []string{"acme/api", "acme/api"}}, "expected GitHub to be asked about [acme/api], got [acme/api acme/api]"},
	}
	for _, tt := range tests {
		if got := tt.problem.String(); got != tt.want {
			t.Errorf("String() = %q, want %q", got, tt.want)
		}
	}
}

func TestFakeGitHubUnreachable(t *testing.T) {
	g := newFakeGitHub()
	g.unreachable = true
	if resp, err := (&http.Client{Transport: g.transport()}).Get(fakeGitHubURL + "/repos/acme/api"); err == nil {
		resp.Body.Close()
		t.Fatalf("got status %d, want a connection error", resp.StatusCode)
	}
	if got := g.askedSoFar(); !slices.Equal(got, []string{"acme/api"}) {
		t.Errorf("asked %v, want [acme/api]", got)
	}
}

func TestFakeGitHubCreatesPR(t *testing.T) {
	g := newFakeGitHub()
	g.createsPR = 42
	request := func(body string) *http.Response {
		req := httptest.NewRequest(http.MethodPost, "/repos/acme/api/pulls", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+fakeGitHubToken)
		response := httptest.NewRecorder()
		g.serve(response, req)
		return response.Result()
	}
	if got := request(`{"head":"gc/run-1","title":"Title"}`).StatusCode; got != http.StatusUnprocessableEntity {
		t.Errorf("status without a base = %d, want %d", got, http.StatusUnprocessableEntity)
	}
	if got := request(`{"head":"gc/run-1","base":"main","title":"Title","body":"Why"}`).StatusCode; got != http.StatusCreated {
		t.Errorf("status = %d, want %d", got, http.StatusCreated)
	}
	want := []createdPR{{Repository: "acme/api", Head: "gc/run-1", Base: "main", Title: "Title", Description: "Why"}}
	if !reflect.DeepEqual(g.created, want) {
		t.Errorf("created = %+v, want %+v", g.created, want)
	}
}

func TestGitHubPRCreatedFindsAnotherPR(t *testing.T) {
	s := newScenario(&spec.Scenario[contract.Fact]{})
	if s.GitHubPRCreated("acme/api", "gc/run-1", "Title", "Why")() == nil {
		t.Error("no PR was created, want a problem")
	}
	s.github.created = []createdPR{{Repository: "acme/api", Head: "gc/run-1", Base: "main", Title: "Other", Description: "Why"}}
	if s.GitHubPRCreated("acme/api", "gc/run-1", "Title", "Why")() == nil {
		t.Error("another PR was created, want a problem")
	}
}

func TestAgentAskedComparesRepositories(t *testing.T) {
	s := newScenario(&spec.Scenario[contract.Fact]{})
	if problem := s.AgentAsked()(); problem != nil {
		t.Errorf("no run and none expected: %v", problem)
	}
	s.agent.asked = []string{"acme/web", "acme/api"}
	if problem := s.AgentAsked("acme/api", "acme/web")(); problem != nil {
		t.Errorf("order must not matter: %v", problem)
	}
	if s.AgentAsked("acme/api")() == nil {
		t.Error("an extra run was started, want a problem")
	}
}
