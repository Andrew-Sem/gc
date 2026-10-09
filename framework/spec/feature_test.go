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
		name         string
		text         string
		want         clauses
		wantProblems []string
	}{
		{"headings group three-level clauses",
			"## A-1. Area\n\n### A-1.1. Requirement\n\nA-1.1.1. Promise.\n\nA-1.1.2. Another.\n",
			clauses{groups: []group{{"A-1", "Area"}, {"A-1.1", "Requirement"}}, leaves: []string{"A-1.1.1", "A-1.1.2"}}, nil},
		{"mention inside a line", "## A-1. Area\n### A-1.1. R\nA-1.1.1. P.\nSee A-1.1.2. for details.\n",
			clauses{groups: []group{{"A-1", "Area"}, {"A-1.1", "R"}}, leaves: []string{"A-1.1.1"}}, nil},
		{"no dot after the id", "A-1.1.1 has no dot.\n", clauses{}, nil},
		{"clause outside its heading", "## A-1. Area\n### A-1.1. R\nA-1.2.1. P.\n",
			clauses{groups: []group{{"A-1", "Area"}, {"A-1.1", "R"}}, leaves: []string{"A-1.2.1"}},
			[]string{"A-1.2.1 is not under heading A-1.2", "A-1.1 has no clauses"}},
		{"requirement outside its area", "## A-1. Area\n### A-2.1. R\nA-2.1.1. P.\n",
			clauses{groups: []group{{"A-1", "Area"}, {"A-2.1", "R"}}, leaves: []string{"A-2.1.1"}},
			[]string{"A-2.1 is not under heading A-2", "A-1 has no clauses"}},
		{"clause with two levels", "## A-1. Area\n### A-1.1. R\nA-1.1.1. P.\nA-1.2. Old style.\n",
			clauses{groups: []group{{"A-1", "Area"}, {"A-1.1", "R"}}, leaves: []string{"A-1.1.1"}},
			[]string{"A-1.2 must be a heading with one or two levels or a clause with three levels"}},
		{"heading with three levels", "## A-1. Area\n### A-1.1. R\nA-1.1.1. P.\n#### A-1.1.2. Deep\n",
			clauses{groups: []group{{"A-1", "Area"}, {"A-1.1", "R"}}, leaves: []string{"A-1.1.1"}},
			[]string{"A-1.1.2 must be a heading with one or two levels or a clause with three levels"}},
		{"duplicate id", "## A-1. Area\n### A-1.1. R\nA-1.1.1. P.\nA-1.1.1. Again.\n",
			clauses{groups: []group{{"A-1", "Area"}, {"A-1.1", "R"}}, leaves: []string{"A-1.1.1"}},
			[]string{"A-1.1.1 appears twice"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, problems := parseClauses(tt.text)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseClauses() = %+v, want %+v", got, tt.want)
			}
			if !reflect.DeepEqual(problems, tt.wantProblems) {
				t.Errorf("parseClauses() problems = %q, want %q", problems, tt.wantProblems)
			}
		})
	}
}

func TestGroupReport(t *testing.T) {
	c := clauses{
		groups: []group{{"A-1", "Area"}, {"A-1.1", "Done"}, {"A-1.2", "Half"}, {"A-1.3", "Missing"}},
		leaves: []string{"A-1.1.1", "A-1.2.1", "A-1.2.2", "A-1.3.1"},
	}
	want := []string{
		"A-1 Area: partial, 2 of 4",
		"A-1.1 Done: complete, 1 of 1",
		"A-1.2 Half: partial, 1 of 2",
		"A-1.3 Missing: none, 0 of 1",
	}
	if got := groupReport(c, []string{"A-1.1.1", "A-1.2.1", "A-1.2.1"}); !reflect.DeepEqual(got, want) {
		t.Errorf("groupReport() = %q, want %q", got, want)
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
		{"report", true, []string{"DEMO-1 Pages: partial, 1 of 2", "DEMO-1.1 Repositories page: partial, 1 of 2", "uncovered: [DEMO-1.1.2]"}, ""},
		{"failing scenario", false, []string{"expected status 200, got 404"}, ""},
		{"unknown clause", false, []string{"clause DEMO-9.1.1 is not in contract testdata/contract.md"}, ""},
		{"heading covered", false, []string{"DEMO-1.1 is a heading in contract testdata/contract.md; cover its clauses instead"}, ""},
		{"malformed contract", false, []string{"contract testdata/malformed.md: DEMO-1.1 has no clauses"}, ""},
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
		f.Run("page opens", func(s *Scenario[string]) { pageOpens(s).Covers("DEMO-1.1.1") }, newApp)
	case "failing scenario":
		f := NewFeature[string](t, "testdata/contract.md")
		f.Run("missing page", func(s *Scenario[string]) {
			s.When(Get("/nowhere")).
				ExpectStatus(http.StatusOK).
				ExpectFacts()
		}, newApp)
	case "unknown clause":
		f := NewFeature[string](t, "testdata/contract.md")
		f.Run("page opens", func(s *Scenario[string]) { pageOpens(s).Covers("DEMO-9.1.1") }, newApp)
	case "heading covered":
		f := NewFeature[string](t, "testdata/contract.md")
		f.Run("page opens", func(s *Scenario[string]) { pageOpens(s).Covers("DEMO-1.1") }, newApp)
	case "malformed contract":
		NewFeature[string](t, "testdata/malformed.md")
	case "missing contract":
		NewFeature[string](t, "testdata/missing.md")
		t.Log("still running")
	}
}
