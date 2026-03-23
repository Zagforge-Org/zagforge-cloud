package cors_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	corsmw "github.com/LegationPro/zagforge/api/internal/middleware/cors"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func TestCors_RestrictedOrigin(t *testing.T) {
	handler := corsmw.Cors([]string{"https://cloud.zagforge.com"})(okHandler())

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/repos", nil)
	req.Header.Set("Origin", "https://cloud.zagforge.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "https://cloud.zagforge.com" {
		t.Errorf("Allow-Origin = %q, want https://cloud.zagforge.com", got)
	}
}

func TestCors_DisallowedOrigin(t *testing.T) {
	handler := corsmw.Cors([]string{"https://cloud.zagforge.com"})(okHandler())

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/repos", nil)
	req.Header.Set("Origin", "https://evil.com")
	req.Header.Set("Access-Control-Request-Method", "GET")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Allow-Origin = %q, want empty for disallowed origin", got)
	}
}

func TestCors_DefaultOrigin(t *testing.T) {
	handler := corsmw.Cors(nil)(okHandler())

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/repos", nil)
	req.Header.Set("Origin", "https://cloud.zagforge.com")
	req.Header.Set("Access-Control-Request-Method", "GET")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "https://cloud.zagforge.com" {
		t.Errorf("Allow-Origin = %q, want https://cloud.zagforge.com (default)", got)
	}
}
