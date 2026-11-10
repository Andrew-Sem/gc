package extensions

import (
	"encoding/base64"
	"net/http"

	"github.com/Andrew-Sem/gc/framework/spec"
	"github.com/Andrew-Sem/gc/specs/contract"
)

const (
	fakeAdminUser     = "spec-operator"
	fakeAdminPassword = "spec-admin-password"
)

var operator = contract.Credentials{User: fakeAdminUser, Password: fakeAdminPassword}

func AdminPassword() string {
	return fakeAdminPassword
}

// Requests of a scenario come from the operator unless they set Authorization themselves.
func asOperator(r *http.Request) {
	if _, set := r.Header["Authorization"]; !set {
		r.SetBasicAuth(fakeAdminUser, fakeAdminPassword)
	}
}

func Anonymously(r spec.Request) spec.Request {
	return r.WithHeader("Authorization", "")
}

func WithLogin(r spec.Request, user, password string) spec.Request {
	return r.WithHeader("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(user+":"+password)))
}

func WithAuthorization(r spec.Request, value string) spec.Request {
	return r.WithHeader("Authorization", value)
}

func operatorGet(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	asOperator(req)
	return http.DefaultClient.Do(req)
}
