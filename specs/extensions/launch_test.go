package extensions

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Andrew-Sem/gc/framework/spec"
)

func TestLaunchEnvironment(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "real-token")
	t.Setenv("GC_SPEC_KEPT", "kept")
	l := newLaunch("gc")
	defer l.github.server.Close()
	l.GivenEnv("GC_ADDR", "no-port").GivenUnset("GITHUB_TOKEN")

	env := l.environ()
	for _, want := range []string{"GC_ADDR=no-port", "GITHUB_API_URL=" + l.github.server.URL, "GC_SPEC_KEPT=kept"} {
		if !slices.Contains(env, want) {
			t.Errorf("environment has no %s", want)
		}
	}
	if i := slices.IndexFunc(env, func(pair string) bool { return strings.HasPrefix(pair, "GITHUB_TOKEN=") }); i >= 0 {
		t.Errorf("environment has %s, want GITHUB_TOKEN unset", env[i])
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
