package admin

import (
	"bytes"
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

func TestListUsers_noClaims_returns403(t *testing.T) {
	h := newTestHandler()
	r := httptest.NewRequest(http.MethodGet, "/auth/admin/users", nil)
	w := httptest.NewRecorder()

	h.ListUsers(w, r)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestGetUser_noClaims_returns403(t *testing.T) {
	h := newTestHandler()
	mux := chi.NewRouter()
	mux.Get("/auth/admin/users/{userID}", h.GetUser)

	r := httptest.NewRequest(http.MethodGet,
		"/auth/admin/users/550e8400-e29b-41d4-a716-446655440000", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestUpdateUser_noClaims_returns403(t *testing.T) {
	h := newTestHandler()
	mux := chi.NewRouter()
	mux.Put("/auth/admin/users/{userID}", h.UpdateUser)

	r := httptest.NewRequest(http.MethodPut,
		"/auth/admin/users/550e8400-e29b-41d4-a716-446655440000",
		bytes.NewBufferString(`{"is_platform_admin":true}`))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestListOrgs_noClaims_returns403(t *testing.T) {
	h := newTestHandler()
	r := httptest.NewRequest(http.MethodGet, "/auth/admin/orgs", nil)
	w := httptest.NewRecorder()

	h.ListOrgs(w, r)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestUpdateOrgPlan_noClaims_returns403(t *testing.T) {
	h := newTestHandler()
	mux := chi.NewRouter()
	mux.Put("/auth/admin/orgs/{orgID}", h.UpdateOrgPlan)

	r := httptest.NewRequest(http.MethodPut,
		"/auth/admin/orgs/550e8400-e29b-41d4-a716-446655440000",
		bytes.NewBufferString(`{"plan":"pro","max_members":50}`))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestParsePagination_defaults(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	limit, offset := parsePagination(r)

	if limit != 50 {
		t.Errorf("expected default limit 50, got %d", limit)
	}
	if offset != 0 {
		t.Errorf("expected default offset 0, got %d", offset)
	}
}

func TestParsePagination_custom(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?limit=25&offset=10", nil)
	limit, offset := parsePagination(r)

	if limit != 25 {
		t.Errorf("expected limit 25, got %d", limit)
	}
	if offset != 10 {
		t.Errorf("expected offset 10, got %d", offset)
	}
}

func TestParsePagination_clamp(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?limit=500", nil)
	limit, _ := parsePagination(r)

	if limit != 50 {
		t.Errorf("expected clamped limit 50, got %d", limit)
	}
}

func TestParsePagination_invalid(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?limit=abc&offset=-5", nil)
	limit, offset := parsePagination(r)

	if limit != 50 {
		t.Errorf("expected default limit 50 for invalid, got %d", limit)
	}
	if offset != 0 {
		t.Errorf("expected default offset 0 for negative, got %d", offset)
	}
}

func TestResponseIsJSON(t *testing.T) {
	h := newTestHandler()
	r := httptest.NewRequest(http.MethodGet, "/auth/admin/users", nil)
	w := httptest.NewRecorder()

	h.ListUsers(w, r)

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected application/json, got %q", ct)
	}
	_, err := httputil.DecodeJSON[httputil.ErrorResponse](w.Body)
	if err != nil {
		t.Errorf("not valid JSON: %v", err)
	}
}
