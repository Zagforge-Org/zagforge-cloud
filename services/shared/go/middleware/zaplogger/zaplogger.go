package zaplogger

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
)

// Middleware returns a Chi-compatible request logger powered by Zap.
// It logs method, path, status, latency, bytes written, and request ID.
func Middleware(log *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			defer func() {
				reqID := middleware.GetReqID(r.Context())
				fields := []zap.Field{
					zap.String("method", r.Method),
					zap.String("path", r.URL.Path),
					zap.Int("status", ww.Status()),
					zap.Duration("latency", time.Since(start)),
					zap.Int("bytes", ww.BytesWritten()),
					zap.String("remote_addr", r.RemoteAddr),
				}
				if reqID != "" {
					fields = append(fields, zap.String("request_id", reqID))
				}
				if q := r.URL.RawQuery; q != "" {
					fields = append(fields, zap.String("query", q))
				}

				status := ww.Status()
				switch {
				case status >= http.StatusInternalServerError:
					log.Error("request completed", fields...)
				case status >= http.StatusBadRequest:
					log.Warn("request completed", fields...)
				default:
					log.Info("request completed", fields...)
				}
			}()

			next.ServeHTTP(ww, r)
		})
	}
}
