package spec

import "net/http"

type Request struct {
	method string
	path   string
}

func Get(path string) Request {
	return Request{method: http.MethodGet, path: path}
}

func Post(path string) Request {
	return Request{method: http.MethodPost, path: path}
}
