package extensions

import (
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Andrew-Sem/gc/framework/spec"
)

func TestLaunchEnvironment(t *testing.T) {
	t.Setenv("GITHUB_APP_ID", "real-app")
	t.Setenv("GC_SPEC_KEPT", "kept")
	l := newLaunch("gc")
	defer l.github.server.Close()
	defer l.removeJournalFile()
	l.GivenEnv("GC_ADDR", "no-port").GivenUnset("GITHUB_APP_ID")

	env := l.environ()
	for _, want := range []string{"GC_ADDR=no-port", "GC_WEBHOOK_SECRET=" + fakeWebhookSecret, "GITHUB_API_URL=" + l.apiURL, "GC_SPEC_KEPT=kept"} {
		if !slices.Contains(env, want) {
			t.Errorf("environment has no %s", want)
		}
	}
	if i := slices.IndexFunc(env, func(pair string) bool { return strings.HasPrefix(pair, "GITHUB_APP_ID=") }); i >= 0 {
		t.Errorf("environment has %s, want GITHUB_APP_ID unset", env[i])
	}
}

func TestEveryKindOfFactIsChecked(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "../contract/facts.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	for _, decl := range file.Decls {
		if method, ok := decl.(*ast.FuncDecl); ok && method.Name.Name == "fact" && method.Recv != nil {
			want = append(want, method.Recv.List[0].Type.(*ast.Ident).Name)
		}
	}
	var got []string
	for _, fact := range EveryKindOfFact() {
		got = append(got, reflect.TypeOf(fact).Name())
	}
	slices.Sort(want)
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Errorf("EveryKindOfFact has %v, the contract has %v", got, want)
	}
}

func TestLaunchRejectsMalformedScenario(t *testing.T) {
	tests := []struct {
		name     string
		describe func(*Launch)
		want     []spec.Problem
	}{
		{"no When", func(l *Launch) { l.ExpectExit() }, []spec.Problem{
			launchProblem("expected one When or WhenStarted call, got 0"),
		}},
		{"no outcome", func(l *Launch) { l.WhenStarted() }, []spec.Problem{
			launchProblem("expected either ExpectExit or ExpectServing/ExpectStatus"),
		}},
		{"request without status", func(l *Launch) { l.When(spec.Get("/repositories")).ExpectServing() }, []spec.Problem{
			launchProblem("expected ExpectStatus for the request"),
		}},
		{"element without a request", func(l *Launch) { l.WhenStarted().ExpectServing().ExpectElement("a") }, []spec.Problem{
			launchProblem("expected a request for ExpectElement or ExpectNoElement"),
		}},
		{"failing write without an exit", func(l *Launch) { l.WhenWriteFails(spec.Get("/repositories")).ExpectServing() }, []spec.Problem{
			launchProblem("expected ExpectExit for a failing write"),
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := newLaunch("gc")
			tt.describe(l)
			if got := l.Run(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("run returned %q, want %q", got, tt.want)
			}
		})
	}
}
