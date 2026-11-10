package web

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"

	"github.com/Andrew-Sem/gc/specs/contract"
)

func operatorOnly(admin contract.Credentials, webhook http.Handler, next http.Handler) http.Handler {
	user, password := sha256.Sum256([]byte(admin.User)), sha256.Sum256([]byte(admin.Password))
	open := admin.User != "" && admin.Password != ""
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/webhooks/github" {
			webhook.ServeHTTP(w, r)
			return
		}
		givenUser, givenPassword, ok := r.BasicAuth()
		gotUser, gotPassword := sha256.Sum256([]byte(givenUser)), sha256.Sum256([]byte(givenPassword))
		userMatches := subtle.ConstantTimeCompare(gotUser[:], user[:])
		passwordMatches := subtle.ConstantTimeCompare(gotPassword[:], password[:])
		if !open || !ok || userMatches&passwordMatches != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="gc", charset="UTF-8"`)
			http.Error(w, "Authentication required", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
