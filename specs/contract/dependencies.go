package contract

import (
	"context"
	"crypto/rsa"
	"net/http"
)

type Journal interface {
	Append(...Fact)
	Facts() []Fact
	Subscribe() (<-chan struct{}, func())
}

type GitHub struct {
	APIURL     string
	AppID      string
	PrivateKey *rsa.PrivateKey
	Transport  http.RoundTripper
}

type AgentChange struct {
	Branch      string
	Base        string
	Title       string
	Description string
}

type TokenUsage struct {
	Input  int
	Cached int
	Output int
}

type AgentResult struct {
	Change      *AgentChange
	Explanation string
	Tokens      TokenUsage
}

type Agent interface {
	Work(ctx context.Context, repository string) (AgentResult, error)
}

type Credentials struct {
	User     string
	Password string
}

type Dependencies struct {
	Journal       Journal
	GitHub        GitHub
	Agent         Agent
	WebhookSecret string
	Admin         Credentials
}
