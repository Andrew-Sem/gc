package contract

import (
	"context"
	"net/http"
)

type Journal interface {
	Append(...Fact)
	Facts() []Fact
}

type GitHub struct {
	APIURL    string
	Token     string
	Transport http.RoundTripper
}

type AgentChange struct {
	Branch      string
	Base        string
	Title       string
	Description string
}

type Agent interface {
	Work(ctx context.Context, repository string) (*AgentChange, error)
}

type Dependencies struct {
	Journal       Journal
	GitHub        GitHub
	Agent         Agent
	WebhookSecret string
}
