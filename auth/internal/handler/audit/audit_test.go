package audit

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/LegationPro/zagforge/auth/internal/db"
	"github.com/LegationPro/zagforge/shared/go/httputil"
)

func newTestHandler() *Handler {
	return NewHandler(&db.DB{}, zap.NewNop())
}

func TestList_noClaims_returns401(t *testing.T) {
	h := newTestHandler()
	mux := chi.NewRouter()
	mux.Get("/auth/orgs/{orgID}/audit-logs", h.List)

	r := httptest.NewRequest(http.MethodGet,
		"/auth/orgs/550e8400-e29b-41d4-a716-446655440000/audit-logs", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestList_invalidOrgID_returns400(t *testing.T) {
	h := newTestHandler()
	mux := chi.NewRouter()
	mux.Get("/auth/orgs/{orgID}/audit-logs", h.List)

	r := httptest.NewRequest(http.MethodGet, "/auth/orgs/not-a-uuid/audit-logs", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestLoginMetrics_noClaims_returns401(t *testing.T) {
	h := newTestHandler()
	mux := chi.NewRouter()
	mux.Get("/auth/orgs/{orgID}/metrics/logins", h.LoginMetrics)

	r := httptest.NewRequest(http.MethodGet,
		"/auth/orgs/550e8400-e29b-41d4-a716-446655440000/metrics/logins", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestFailedLoginMetrics_invalidOrgID_returns400(t *testing.T) {
	h := newTestHandler()
	mux := chi.NewRouter()
	mux.Get("/auth/orgs/{orgID}/metrics/failed-logins", h.FailedLoginMetrics)

	r := httptest.NewRequest(http.MethodGet, "/auth/orgs/bad/metrics/failed-logins", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestResponseIsJSON(t *testing.T) {
	h := newTestHandler()
	mux := chi.NewRouter()
	mux.Get("/auth/orgs/{orgID}/audit-logs", h.List)

	r := httptest.NewRequest(http.MethodGet, "/auth/orgs/bad/audit-logs", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}
	_, err := httputil.DecodeJSON[httputil.ErrorResponse](w.Body)
	if err != nil {
		t.Errorf("not valid JSON: %v", err)
	}
}

func TestParseDateRange_defaults(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	from, to := parseDateRange(r)

	if !from.Valid || !to.Valid {
		t.Fatal("expected valid date range")
	}
	if to.Time.Before(from.Time) {
		t.Error("expected to >= from")
	}
}

func TestParseDateRange_custom(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?from=2026-01-01&to=2026-01-31", nil)
	from, to := parseDateRange(r)

	if from.Time.Year() != 2026 || from.Time.Month() != 1 || from.Time.Day() != 1 {
		t.Errorf("unexpected from: %v", from.Time)
	}
	if to.Time.Year() != 2026 || to.Time.Month() != 1 || to.Time.Day() != 31 {
		t.Errorf("unexpected to: %v", to.Time)
	}
}
