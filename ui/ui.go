package ui

import (
	"embed"
	"net/http"
)

//go:embed static
var static embed.FS

const (
	HTMXVersion = "2.0.11"
	HTMXPath    = "/static/htmx.min.js"
	HTMXConfig  = `{"responseHandling":[` +
		`{"code":"204","swap":false},` +
		`{"code":"[23]..","swap":true},` +
		`{"code":"422","swap":true},` +
		`{"code":"502","swap":true,"error":true},` +
		`{"code":"[45]..","swap":false,"error":true}]}`
)

func Static() http.Handler {
	return http.FileServerFS(static)
}
