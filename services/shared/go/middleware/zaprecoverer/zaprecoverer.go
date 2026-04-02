package zaprecoverer

import (
	"net/http"
	"runtime/debug"

	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
)

// Middleware returns a Chi-compatible panic recoverer that logs via Zap
// instead of writing to stderr. Returns 500 on panic.
func Middleware(log *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					reqID := middleware.GetReqID(r.Context())
					fields := []zap.Field{
						zap.Any("panic", rec),
						zap.String("method", r.Method),
						zap.String("path", r.URL.Path),
						zap.String("stack", string(debug.Stack())),
					}
					if reqID != "" {
						fields = append(fields, zap.String("request_id", reqID))
					}
					log.Error("panic recovered", fields...)

					if r.Header.Get("Connection") != "Upgrade" {
						w.WriteHeader(http.StatusInternalServerError)
					}
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
