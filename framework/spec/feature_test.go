package spec

import (
	"errors"
	"net/http"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"github.com/Andrew-Sem/gc/framework/journal"
)

func TestParseClauses(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []string
	}{
		{"clauses at line start", "REPO-1. Page.\n\nREPO-1.1. Connect.\n", []string{"REPO-1", "REPO-1.1"}},
		{"deep numbering", "REPO-1.2.3. Deep.\n", []string{"REPO-1.2.3"}},
		{"mention inside a line", "See REPO-1. for details.\n", nil},
		{"no dot after the id", "REPO-1 has no dot.\n", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseClauses(tt.text); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseClauses() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestUncovered(t *testing.T) {
	tests := []struct {
		name    string
		clauses []string
		covered []string
		want    []string
	}{
		{"some covered", []string{"A-1", "A-2", "A-3"}, []string{"A-2"}, []string{"A-1", "A-3"}},
		{"all covered", []string{"A-1"}, []string{"A-1"}, nil},
		{"covered twice", []string{"A-1", "A-2"}, []string{"A-1", "A-1"}, []string{"A-2"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := uncovered(tt.clauses, tt.covered); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("uncovered() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFeatureReportsThroughGoTest(t *testing.T) {
	tests := []struct {
		helper     string
		wantPass   bool
		wantOutput []string
		notOutput  string
	}{
		{"report", true, []string{"uncovered: [DEMO-2]", "covered: [DEMO-1]"}, ""},
		{"failing scenario", false, []string{"expected status 200, got 404"}, ""},
		{"unknown clause", false, []string{"clause DEMO-9 is not in contract testdata/contract.md"}, ""},
		{"missing contract", false, []string{"cannot read contract"}, "still running"},
	}
	for _, tt := range tests {
		t.Run(tt.helper, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestFeatureHelper$", "-test.v")
			cmd.Env = append(os.Environ(), "GC_FEATURE_HELPER="+tt.helper)
			out, err := cmd.CombinedOutput()

			var exit *exec.ExitError
			if err != nil && !errors.As(err, &exit) {
				t.Fatalf("cannot start the test binary: %v", err)
			}
			if passed := err == nil; passed != tt.wantPass {
				t.Errorf("passed = %v, want %v; output:\n%s", passed, tt.wantPass, out)
			}
			for _, want := range tt.wantOutput {
				if !strings.Contains(string(out), want) {
					t.Errorf("output has no %q:\n%s", want, out)
				}
			}
			if tt.notOutput != "" && strings.Contains(string(out), tt.notOutput) {
				t.Errorf("output has %q:\n%s", tt.notOutput, out)
			}
		})
	}
}

func TestFeatureHelper(t *testing.T) {
	helper := os.Getenv("GC_FEATURE_HELPER")
	if helper == "" {
		t.Skip("runs only as a subprocess of TestFeatureReportsThroughGoTest")
	}
	newApp := func(*journal.Journal[string]) http.Handler {
		mux := http.NewServeMux()
		mux.HandleFunc("GET /repositories", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		return mux
	}
	pageOpens := func(s *Scenario[string]) *Scenario[string] {
		return s.When(Get("/repositories")).
			ExpectStatus(http.StatusOK).
			ExpectFacts()
	}
	switch helper {
	case "report":
		f := NewFeature[string](t, "testdata/contract.md")
		f.Run("page opens", func(s *Scenario[string]) { pageOpens(s).Covers("DEMO-1") }, newApp)
	case "failing scenario":
		f := NewFeature[string](t, "testdata/contract.md")
		f.Run("missing page", func(s *Scenario[string]) {
			s.When(Get("/nowhere")).
				ExpectStatus(http.StatusOK).
				ExpectFacts()
		}, newApp)
	case "unknown clause":
		f := NewFeature[string](t, "testdata/contract.md")
		f.Run("page opens", func(s *Scenario[string]) { pageOpens(s).Covers("DEMO-9") }, newApp)
	case "missing contract":
		NewFeature[string](t, "testdata/missing.md")
		t.Log("still running")
	}
}
