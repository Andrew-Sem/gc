package spec

import (
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/Andrew-Sem/gc/internal/journal"
	"github.com/Andrew-Sem/gc/internal/web"
)

type Feature struct {
	t        *testing.T
	contract string
}

func NewFeature(t *testing.T, contract string) *Feature {
	return &Feature{t: t, contract: contract}
}

func (f *Feature) Run(name string, describe func(s *Scenario)) {
	f.t.Run(name, func(t *testing.T) {
		s := &Scenario{}
		describe(s)
		j := &journal.Journal{}
		j.Append(s.given...)
		before := len(j.Facts())
		response := httptest.NewRecorder()
		web.New(j).ServeHTTP(response, httptest.NewRequest(s.request.method, s.request.path, nil))
		if s.status != 0 && response.Code != s.status {
			t.Errorf("ждали статус %d, пришёл %d", s.status, response.Code)
		}
		written := j.Facts()[before:]
		if !slices.Equal(written, s.expected) {
			t.Errorf("ждали факты %#v, записано %#v", s.expected, written)
		}
	})
}
