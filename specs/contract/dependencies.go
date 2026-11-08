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

type Agent interface {
	Work(ctx context.Context, repository string) (*AgentChange, error)
}

type Dependencies struct {
	Journal       Journal
	GitHub        GitHub
	Agent         Agent
	WebhookSecret string
}
