package spec

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
)

type Request struct {
	method string
	path   string
	form   url.Values
}

func Get(path string) Request {
	return Request{method: http.MethodGet, path: path}
}

func Post(path string) Request {
	return Request{method: http.MethodPost, path: path}
}

func (r Request) WithForm(key, value string) Request {
	if r.form == nil {
		r.form = url.Values{}
	}
	r.form.Add(key, value)
	return r
}

func (r Request) httpRequest() *http.Request {
	request := httptest.NewRequest(r.method, r.path, strings.NewReader(r.form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return request
}
