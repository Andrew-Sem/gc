package web

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/Andrew-Sem/gc/internal/journal"
)

func TestServesHtmx(t *testing.T) {
	response := httptest.NewRecorder()
	New(&journal.Journal{}, nil).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/static/htmx.min.js", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status %d, want %d", response.Code, http.StatusOK)
	}
	if got := response.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/javascript") {
		t.Errorf("Content-Type %q, want text/javascript", got)
	}
	if body, _ := io.ReadAll(response.Body); !strings.HasPrefix(string(body), "var htmx=") {
		t.Errorf("body does not start with htmx")
	}
}

type responseRule struct {
	Code  string `json:"code"`
	Swap  bool   `json:"swap"`
	Error bool   `json:"error"`
}

func ruleFor(rules []responseRule, status int) responseRule {
	for _, rule := range rules {
		if regexp.MustCompile(rule.Code).MatchString(strconv.Itoa(status)) {
			return rule
		}
	}
	return responseRule{}
}

func TestHtmxConfigSwapsPanelResponses(t *testing.T) {
	var config struct {
		ResponseHandling []responseRule `json:"responseHandling"`
	}
	if err := json.Unmarshal([]byte(htmxConfig), &config); err != nil {
		t.Fatalf("htmx config is not JSON: %v", err)
	}
	tests := []struct {
		status int
		want   responseRule
	}{
		{http.StatusOK, responseRule{Code: "[23]..", Swap: true}},
		{http.StatusNoContent, responseRule{Code: "204"}},
		{http.StatusUnprocessableEntity, responseRule{Code: "422", Swap: true}},
		{http.StatusBadGateway, responseRule{Code: "502", Swap: true, Error: true}},
		{http.StatusNotFound, responseRule{Code: "[45]..", Error: true}},
		{http.StatusInternalServerError, responseRule{Code: "[45]..", Error: true}},
	}
	for _, tt := range tests {
		t.Run(http.StatusText(tt.status), func(t *testing.T) {
			if got := ruleFor(config.ResponseHandling, tt.status); got != tt.want {
				t.Errorf("rule for %d is %+v, want %+v", tt.status, got, tt.want)
			}
		})
	}
}
