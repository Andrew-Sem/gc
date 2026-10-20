package spec

import (
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
)

type Request struct {
	method string
	path   string
	form   url.Values
	header http.Header
	body   string
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

func (r Request) WithHeader(name, value string) Request {
	r.header = r.header.Clone()
	if r.header == nil {
		r.header = http.Header{}
	}
	r.header.Set(name, value)
	return r
}

func (r Request) WithBody(contentType, body string) Request {
	r.body = body
	return r.WithHeader("Content-Type", contentType)
}

func (r Request) content() (io.Reader, http.Header) {
	header := http.Header{"Content-Type": {"application/x-www-form-urlencoded"}}
	body := r.form.Encode()
	if r.body != "" {
		body = r.body
	}
	maps.Copy(header, r.header)
	return strings.NewReader(body), header
}

func (r Request) httpRequest() *http.Request {
	body, header := r.content()
	request := httptest.NewRequest(r.method, r.path, body)
	request.Header = header
	return request
}

func (r Request) HTTPRequest(baseURL string) (*http.Request, error) {
	body, header := r.content()
	request, err := http.NewRequest(r.method, baseURL+r.path, body)
	if err != nil {
		return nil, err
	}
	request.Header = header
	return request, nil
}
