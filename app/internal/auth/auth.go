// Package auth provides a bearer-token guard for /metrics and /admin.
package auth

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"strings"
)

// Bearer returns next unchanged when token is empty (local dev). Otherwise it
// requires "Authorization: Bearer <token>". Both sides are hashed first so the
// comparison is constant-time regardless of the length of what the client sent.
func Bearer(token string, next http.Handler) http.Handler {
	if token == "" {
		return next
	}
	want := sha256.Sum256([]byte(token))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := r.Header.Get("Authorization")
		got := sha256.Sum256([]byte(strings.TrimPrefix(h, "Bearer ")))
		if !strings.HasPrefix(h, "Bearer ") || subtle.ConstantTimeCompare(got[:], want[:]) != 1 {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
