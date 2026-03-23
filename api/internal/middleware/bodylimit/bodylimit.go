package bodylimit

import (
	"errors"
	"net/http"

	"github.com/LegationPro/zagforge/shared/go/httputil"
)

var errBodyTooLarge = errors.New("request body too large")

// Limit returns middleware that restricts request body size to maxBytes.
// Applies only to methods that carry a body (POST, PUT, PATCH).
func Limit(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil && hasBody(r.Method) {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			}
			next.ServeHTTP(w, r)
		})
	}
}

func hasBody(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch:
		return true
	default:
		return false
	}
}

// HandleMaxBytesError checks if err is a max-bytes exceeded error and writes a 413 if so.
// Returns true if handled.
func HandleMaxBytesError(w http.ResponseWriter, err error) bool {
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		httputil.ErrResponse(w, http.StatusRequestEntityTooLarge, errBodyTooLarge)
		return true
	}
	return false
}
