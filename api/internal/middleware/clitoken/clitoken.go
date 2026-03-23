package clitoken

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"

	"github.com/LegationPro/zagforge/shared/go/httputil"
)

var (
	errMissing = errors.New("missing CLI API key")
	errInvalid = errors.New("invalid CLI API key")
)

// Auth returns middleware that validates a static CLI bearer token.
// Uses constant-time comparison to prevent timing attacks.
func Auth(validKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, found := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			if !found || token == "" {
				httputil.ErrResponse(w, http.StatusUnauthorized, errMissing)
				return
			}

			if subtle.ConstantTimeCompare([]byte(token), []byte(validKey)) != 1 {
				httputil.ErrResponse(w, http.StatusUnauthorized, errInvalid)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
