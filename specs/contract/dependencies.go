package contract

import "net/http"

type Journal interface {
	Append(...Fact)
	Facts() []Fact
}

type GitHub struct {
	APIURL    string
	Token     string
	Transport http.RoundTripper
}

type Dependencies struct {
	Journal Journal
	GitHub  GitHub
}
