package domain

import (
	"net/url"
	"strings"

	"github.com/Andrew-Sem/gc/specs/contract"
)

func NormalizeBaseURL(submitted string) (string, bool) {
	trimmed := strings.TrimSpace(submitted)
	if trimmed == "" {
		return "", true
	}
	if strings.ContainsAny(trimmed, "?#") {
		return trimmed, false
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil {
		return trimmed, false
	}
	return parsed.Scheme + "://" + strings.ToLower(parsed.Host) + strings.TrimRight(parsed.EscapedPath(), "/"), true
}

func OpenAIBaseURL(facts []contract.Fact, fallback string) string {
	value := fallback
	for _, f := range facts {
		if changed, ok := f.(contract.SettingsChanged); ok {
			value = changed.OpenAIBaseURL
		}
	}
	return value
}
