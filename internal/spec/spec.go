package spec

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Andrew-Sem/gc/internal/web"
)

type Request struct {
	method string
	path   string
}

func Run(t *testing.T, name string, describe func(s *Scenario)) {
	t.Run(name, func(t *testing.T) {
		s := &Scenario{}
		describe(s)
		response := httptest.NewRecorder()
		web.New().ServeHTTP(response, httptest.NewRequest(s.request.method, s.request.path, nil))
		if response.Code != s.status {
			t.Errorf("ждали статус %d, пришёл %d", s.status, response.Code)
		}
	})
}

func Get(path string) Request {
	return Request{method: http.MethodGet, path: path}
}
