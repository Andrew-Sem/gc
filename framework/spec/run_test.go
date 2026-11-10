package spec

import (
	"fmt"
	"net/http"
	"reflect"
	"testing"

	"github.com/Andrew-Sem/gc/framework/journal"
)

func TestRunFindsNothingInMatchingScenario(t *testing.T) {
	disconnected := removed{Name: "acme/api"}

	s := &Scenario[fact]{}
	s.When(Post("/repositories/acme/api/disconnect")).
		ExpectStatus(http.StatusOK).
		ExpectFacts(disconnected)

	writesExpectedFact := func(j *journal.Journal[fact]) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			j.Append(disconnected)
			w.WriteHeader(http.StatusOK)
		})
	}

	if problems := s.Run(writesExpectedFact); len(problems) != 0 {
		t.Errorf("run returned %q, want no problems", problems)
	}
}

type customProblem string

func (p customProblem) String() string { return string(p) }

func TestRunReportsFailedExpectations(t *testing.T) {
	s := &Scenario[fact]{}
	s.When(Get("/repositories")).
		ExpectStatus(http.StatusOK).
		ExpectFacts().
		Expect(func() Problem { return nil }).
		Expect(func() Problem { return customProblem("the check failed") })

	ok := func(*journal.Journal[fact]) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	}

	problems := s.Run(ok)
	want := []Problem{customProblem("the check failed")}
	if !reflect.DeepEqual(problems, want) {
		t.Errorf("run returned %q, want %q", problems, want)
	}
}

func TestRunFindsMissingRoute(t *testing.T) {
	s := &Scenario[fact]{}
	s.When(Post("/repositories/acme/api/disconnect")).
		ExpectStatus(http.StatusOK).
		ExpectFacts()

	noRoutes := func(*journal.Journal[fact]) http.Handler {
		return http.NewServeMux()
	}

	problems := s.Run(noRoutes)
	want := []Problem{statusMismatch{expected: http.StatusOK, got: http.StatusNotFound}}
	if !reflect.DeepEqual(problems, want) {
		t.Errorf("run returned %q, want %q", problems, want)
	}
}

func TestRunFindsWrongFacts(t *testing.T) {
	connected := added{Name: "acme/api"}
	disconnected := removed{Name: "acme/api"}

	s := &Scenario[fact]{}
	s.When(Post("/repositories/acme/api/disconnect")).
		ExpectStatus(http.StatusOK).
		ExpectFacts(disconnected)

	writesWrongFact := func(j *journal.Journal[fact]) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			j.Append(connected)
			w.WriteHeader(http.StatusOK)
		})
	}

	problems := s.Run(writesWrongFact)
	want := []Problem{factsMismatch[fact]{expected: []fact{disconnected}, got: []fact{connected}}}
	if !reflect.DeepEqual(problems, want) {
		t.Errorf("run returned %q, want %q", problems, want)
	}
}

func TestFactList(t *testing.T) {
	tests := []struct {
		name  string
		facts []fact
		want  string
	}{
		{"no facts", nil, "[]"},
		{"one fact", []fact{
			added{Name: "acme/api"},
		}, "[spec.added{Name:acme/api}]"},
		{"two facts", []fact{
			added{Name: "acme/api"},
			removed{Name: "acme/api"},
		}, "[spec.added{Name:acme/api}, spec.removed{Name:acme/api}]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := factList(tt.facts...); got != tt.want {
				t.Errorf("factList() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRunReportsStatusThenFacts(t *testing.T) {
	connected := added{Name: "acme/api"}
	disconnected := removed{Name: "acme/api"}

	s := &Scenario[fact]{}
	s.When(Post("/repositories/acme/api/disconnect")).
		ExpectStatus(http.StatusOK).
		ExpectFacts(disconnected)

	failsAndWritesWrongFact := func(j *journal.Journal[fact]) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			j.Append(connected)
			w.WriteHeader(http.StatusInternalServerError)
		})
	}

	problems := s.Run(failsAndWritesWrongFact)
	want := []Problem{
		statusMismatch{expected: http.StatusOK, got: http.StatusInternalServerError},
		factsMismatch[fact]{expected: []fact{disconnected}, got: []fact{connected}},
	}
	if !reflect.DeepEqual(problems, want) {
		t.Errorf("run returned %q, want %q", problems, want)
	}
}

func TestRunAcceptsExplicitlyEmptyFacts(t *testing.T) {
	s := &Scenario[fact]{}
	s.When(Get("/repositories")).
		ExpectStatus(http.StatusOK).
		ExpectFacts()

	writesNothing := func(*journal.Journal[fact]) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
	}

	if problems := s.Run(writesNothing); len(problems) != 0 {
		t.Errorf("run returned %q, want no problems", problems)
	}
}

func TestRunRejectsMalformedScenario(t *testing.T) {
	tests := []struct {
		name     string
		describe func(s *Scenario[fact])
		want     []Problem
	}{
		{"no When", func(s *Scenario[fact]) {
			s.ExpectStatus(http.StatusOK).ExpectFacts()
		}, []Problem{callCountMismatch{method: "When", got: 0}}},
		{"two Whens", func(s *Scenario[fact]) {
			s.When(Get("/repositories")).
				When(Get("/repositories")).
				ExpectStatus(http.StatusOK).
				ExpectFacts()
		}, []Problem{callCountMismatch{method: "When", got: 2}}},
		{"no ExpectStatus", func(s *Scenario[fact]) {
			s.When(Get("/repositories")).ExpectFacts()
		}, []Problem{callCountMismatch{method: "ExpectStatus", got: 0}}},
		{"no ExpectFacts", func(s *Scenario[fact]) {
			s.When(Get("/repositories")).ExpectStatus(http.StatusOK)
		}, []Problem{callCountMismatch{method: "ExpectFacts", got: 0}}},
		{"nothing at all", func(s *Scenario[fact]) {}, []Problem{
			callCountMismatch{method: "When", got: 0},
			callCountMismatch{method: "ExpectStatus", got: 0},
			callCountMismatch{method: "ExpectFacts", got: 0},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Scenario[fact]{}
			tt.describe(s)

			mustNotStart := func(*journal.Journal[fact]) http.Handler {
				t.Error("run started the app for a malformed scenario")
				return http.NewServeMux()
			}

			if problems := s.Run(mustNotStart); !reflect.DeepEqual(problems, tt.want) {
				t.Errorf("run returned %q, want %q", problems, tt.want)
			}
		})
	}
}

func TestProblemString(t *testing.T) {
	tests := []struct {
		name    string
		problem Problem
		want    string
	}{
		{"status mismatch", statusMismatch{expected: 200, got: 404}, "expected status 200, got 404"},
		{"facts mismatch", factsMismatch[fact]{
			expected: []fact{removed{Name: "acme/api"}},
			got:      nil,
		}, "expected facts [spec.removed{Name:acme/api}], got []"},
		{"call count mismatch", callCountMismatch{method: "When", got: 2}, "expected one When call, got 2"},
		{"element missing", elementMissing{selector: "form"}, "expected element form, found none"},
		{"invalid selector", invalidSelector{selector: "form[", err: "boom"}, "invalid selector form[: boom"},
		{"unreadable page", unreadablePage{err: "boom"}, "cannot read the page: boom"},
		{"unexpected element", unexpectedElement{selector: "form"}, "expected no element form, found one"},
		{"text mismatch", textMismatch{selector: "p", expected: "yes", got: "no"}, `expected text "yes" in p, got "no"`},
		{"ambiguous element", ambiguousElement{selector: "p", got: 2}, "expected one element p, found 2"},
		{"body contains", bodyContains{text: "secret", got: "page"}, `expected the response not to contain "secret", got "page"`},
		{"body missing", bodyMissing{text: "secret", got: "page"}, `expected the response to contain "secret", got "page"`},
		{"element count mismatch", elementCountMismatch{selector: "li", expected: 1, got: 2}, "expected 1 elements li, found 2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.problem.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func servesForm(*journal.Journal[fact]) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<form><input name="repo"></form>`)
	})
}

func TestRunChecksElements(t *testing.T) {
	tests := []struct {
		name     string
		selector string
		want     []Problem
	}{
		{"element found", `form input[name="repo"]`, nil},
		{"element missing", `form input[name="owner"]`, []Problem{
			elementMissing{selector: `form input[name="owner"]`},
		}},
		{"invalid selector", `form[`, []Problem{
			invalidSelector{selector: `form[`, err: "expected identifier, found EOF instead"},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Scenario[fact]{}
			s.When(Get("/repositories")).
				ExpectStatus(http.StatusOK).
				ExpectFacts().
				ExpectElement(tt.selector)

			if problems := s.Run(servesForm); !reflect.DeepEqual(problems, tt.want) {
				t.Errorf("run returned %q, want %q", problems, tt.want)
			}
		})
	}
}

func TestRunChecksAbsentElements(t *testing.T) {
	tests := []struct {
		name     string
		selector string
		want     []Problem
	}{
		{"element absent", `form input[name="owner"]`, nil},
		{"element present", `form input[name="repo"]`, []Problem{
			unexpectedElement{selector: `form input[name="repo"]`},
		}},
		{"invalid selector", `form[`, []Problem{
			invalidSelector{selector: `form[`, err: "expected identifier, found EOF instead"},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Scenario[fact]{}
			s.When(Get("/repositories")).
				ExpectStatus(http.StatusOK).
				ExpectFacts().
				ExpectNoElement(tt.selector)

			if problems := s.Run(servesForm); !reflect.DeepEqual(problems, tt.want) {
				t.Errorf("run returned %q, want %q", problems, tt.want)
			}
		})
	}
}

func servesAlert(*journal.Journal[fact]) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<div role="alert">  No
			<b>access</b> </div><p>one</p><p>two</p>`)
	})
}

func TestRunChecksText(t *testing.T) {
	tests := []struct {
		name     string
		selector string
		text     string
		want     []Problem
	}{
		{"text matches", `[role="alert"]`, "No access", nil},
		{"text differs", `[role="alert"]`, "No connection", []Problem{
			textMismatch{selector: `[role="alert"]`, expected: "No connection", got: "No access"},
		}},
		{"element missing", `#missing`, "No access", []Problem{
			elementMissing{selector: `#missing`},
		}},
		{"several elements", `p`, "one", []Problem{
			ambiguousElement{selector: `p`, got: 2},
		}},
		{"invalid selector", `form[`, "No access", []Problem{
			invalidSelector{selector: `form[`, err: "expected identifier, found EOF instead"},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Scenario[fact]{}
			s.When(Get("/repositories")).
				ExpectStatus(http.StatusOK).
				ExpectFacts().
				ExpectText(tt.selector, tt.text)

			if problems := s.Run(servesAlert); !reflect.DeepEqual(problems, tt.want) {
				t.Errorf("run returned %q, want %q", problems, tt.want)
			}
		})
	}
}

type fact interface{ fact() }
type added struct{ Name string }
type removed struct{ Name string }

func (added) fact()   {}
func (removed) fact() {}

func TestRunChecksBodyWithout(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []Problem
	}{
		{"text absent", "secret", nil},
		{"text in an attribute", `name="repo"`, []Problem{bodyContains{text: `name="repo"`, got: `<form><input name="repo"></form>`}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Scenario[fact]{}
			s.When(Get("/repositories")).
				ExpectStatus(http.StatusOK).
				ExpectFacts().
				ExpectBodyWithout(tt.text)

			if problems := s.Run(servesForm); !reflect.DeepEqual(problems, tt.want) {
				t.Errorf("run returned %q, want %q", problems, tt.want)
			}
		})
	}
}

func TestRunChecksElementCount(t *testing.T) {
	tests := []struct {
		name     string
		selector string
		count    int
		want     []Problem
	}{
		{"count matches", "p", 2, nil},
		{"none expected", "li", 0, nil},
		{"count differs", "p", 1, []Problem{elementCountMismatch{selector: "p", expected: 1, got: 2}}},
		{"invalid selector", `p[`, 1, []Problem{
			invalidSelector{selector: `p[`, err: "expected identifier, found EOF instead"},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Scenario[fact]{}
			s.When(Get("/repositories")).
				ExpectStatus(http.StatusOK).
				ExpectFacts().
				ExpectElementCount(tt.selector, tt.count)

			if problems := s.Run(servesAlert); !reflect.DeepEqual(problems, tt.want) {
				t.Errorf("run returned %q, want %q", problems, tt.want)
			}
		})
	}
}

func TestRunChecksBodyContaining(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []Problem
	}{
		{"text present", `name="repo"`, nil},
		{"text missing", "secret", []Problem{bodyMissing{text: "secret", got: `<form><input name="repo"></form>`}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Scenario[fact]{}
			s.When(Get("/repositories")).
				ExpectStatus(http.StatusOK).
				ExpectFacts().
				ExpectBodyContaining(tt.text)

			if problems := s.Run(servesForm); !reflect.DeepEqual(problems, tt.want) {
				t.Errorf("run returned %q, want %q", problems, tt.want)
			}
		})
	}
}

func TestActionScenarioChecksNewFacts(t *testing.T) {
	act := func(j *journal.Journal[string]) { j.Append("woken") }
	tests := []struct {
		name     string
		expected []string
		want     []Problem
	}{
		{"expected fact", []string{"woken"}, nil},
		{"other fact", []string{"other"}, []Problem{factsMismatch[string]{expected: []string{"other"}, got: []string{"woken"}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Scenario[string]{}
			s.Given("before").WhenAction(act).ExpectFacts(tt.expected...)
			if got := s.Run(nil); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Run() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestActionScenarioRunsChecks(t *testing.T) {
	s := &Scenario[string]{}
	s.WhenAction(func(*journal.Journal[string]) {}).ExpectFacts().Expect(func() Problem { return customProblem("the check failed") })
	want := []Problem{customProblem("the check failed")}
	if got := s.Run(nil); !reflect.DeepEqual(got, want) {
		t.Errorf("Run() = %v, want %v", got, want)
	}
}

func TestActionScenarioRejectsResponseChecks(t *testing.T) {
	tests := []struct {
		name  string
		check func(*Scenario[string])
	}{
		{"status", func(s *Scenario[string]) { s.ExpectStatus(200) }},
		{"element", func(s *Scenario[string]) { s.ExpectElement("form") }},
		{"no element", func(s *Scenario[string]) { s.ExpectNoElement("form") }},
		{"text", func(s *Scenario[string]) { s.ExpectText("p", "yes") }},
		{"count", func(s *Scenario[string]) { s.ExpectElementCount("li", 1) }},
		{"body contains", func(s *Scenario[string]) { s.ExpectBodyContaining("yes") }},
		{"body without", func(s *Scenario[string]) { s.ExpectBodyWithout("no") }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Scenario[string]{}
			s.WhenAction(func(*journal.Journal[string]) { t.Error("the action ran") }).ExpectFacts()
			tt.check(s)
			want := []Problem{noResponse{}}
			if got := s.Run(nil); !reflect.DeepEqual(got, want) {
				t.Errorf("Run() = %v, want %v", got, want)
			}
		})
	}
}

func TestActionScenarioNeedsFacts(t *testing.T) {
	s := &Scenario[string]{}
	s.WhenAction(func(*journal.Journal[string]) {})
	want := []Problem{callCountMismatch{method: "ExpectFacts", got: 0}}
	if got := s.Run(nil); !reflect.DeepEqual(got, want) {
		t.Errorf("Run() = %v, want %v", got, want)
	}
}

func TestGivenAddsFacts(t *testing.T) {
	var got []string
	s := &Scenario[string]{}
	s.Given("first").Given("second", "third").
		WhenAction(func(j *journal.Journal[string]) { got = j.Facts() }).
		ExpectFacts()
	if problems := s.Run(nil); len(problems) != 0 {
		t.Fatalf("run returned %q, want no problems", problems)
	}
	if want := []string{"first", "second", "third"}; !reflect.DeepEqual(got, want) {
		t.Errorf("journal = %v, want %v", got, want)
	}
}

func TestGivenRequestReachesTheSameApplicationAndIsNotCheckedAsNewFacts(t *testing.T) {
	var apps int
	countsRequests := func(j *journal.Journal[string]) http.Handler {
		apps++
		seen := 0
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			seen++
			j.Append(fmt.Sprintf("request %d", seen))
		})
	}
	s := &Scenario[string]{}
	s.GivenRequest(Get("/first")).
		When(Get("/second")).
		ExpectStatus(http.StatusOK).
		ExpectFacts("request 2")
	if got := s.Run(countsRequests); len(got) != 0 {
		t.Errorf("Run() = %v, want no problems", got)
	}
	if apps != 1 {
		t.Errorf("built %d applications, want 1", apps)
	}
}

func TestActionScenarioRejectsGivenRequest(t *testing.T) {
	s := &Scenario[string]{}
	s.GivenRequest(Get("/first")).WhenAction(func(*journal.Journal[string]) {}).ExpectFacts()
	want := []Problem{requestBeforeAction{}}
	if got := s.Run(nil); !reflect.DeepEqual(got, want) {
		t.Errorf("Run() = %v, want %v", got, want)
	}
}

type listed struct {
	Names []string
}

func (listed) fact() {}

func TestRunComparesFactsThatHoldLists(t *testing.T) {
	for _, tt := range []struct {
		name    string
		written listed
		want    int
	}{
		{"the same list", listed{Names: []string{"test", "lint"}}, 0},
		{"another list", listed{Names: []string{"test"}}, 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := &Scenario[fact]{}
			s.WhenAction(func(j *journal.Journal[fact]) { j.Append(tt.written) }).
				ExpectFacts(listed{Names: []string{"test", "lint"}})
			if problems := s.Run(nil); len(problems) != tt.want {
				t.Errorf("run returned %q, want %d problems", problems, tt.want)
			}
		})
	}
}
