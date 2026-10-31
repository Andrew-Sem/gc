package spec

import (
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/Andrew-Sem/gc/framework/journal"
)

func TestVirtualScenariosWaitForParent(t *testing.T) {
	var registered atomic.Bool
	var called atomic.Int32
	f := NewFeature[string](t, "testdata/contract.md")
	for i := range 32 {
		f.Run(fmt.Sprint(i), func(s *Scenario[string]) {
			s.Covers("DEMO-1.1.1").When(Get("/")).ExpectStatus(http.StatusOK).ExpectFacts()
		}, func(*journal.Journal[string]) http.Handler {
			called.Add(1)
			return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if !registered.Load() {
					w.WriteHeader(http.StatusInternalServerError)
				}
			})
		})
	}
	if called.Load() != 0 {
		t.Errorf("ran %d scenarios before parent returned, want 0", called.Load())
	}
	registered.Store(true)
	t.Cleanup(func() {
		if called.Load() != 32 || len(f.covered) != 32 || len(f.failed) != 0 {
			t.Errorf("calls/covered/failed = %d/%d/%d, want 32/32/0", called.Load(), len(f.covered), len(f.failed))
		}
	})
}

func TestCustomScenariosStaySequential(t *testing.T) {
	f := NewFeature[string](t, "testdata/contract.md")
	called := false
	f.RunCustom("real resources", func() Custom {
		called = true
		return custom{covers: []string{"DEMO-1.1.1"}}
	})
	if !called {
		t.Fatal("custom scenario did not finish before RunCustom returned")
	}
}
