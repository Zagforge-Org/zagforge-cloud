package webhook

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/LegationPro/zagforge/auth/internal/db"
	"github.com/LegationPro/zagforge/shared/go/authclaims"
	"github.com/LegationPro/zagforge/shared/go/httputil"
)

func newTestHandler() *Handler {
	return NewHandler(&db.DB{}, zap.NewNop())
}

func TestCreate_noClaims_returns401(t *testing.T) {
	h := newTestHandler()
	mux := chi.NewRouter()
	mux.Post("/auth/orgs/{orgID}/webhooks", h.Create)

	r := httptest.NewRequest(http.MethodPost,
		"/auth/orgs/550e8400-e29b-41d4-a716-446655440000/webhooks",
		bytes.NewBufferString(`{"url":"https://example.com/hook","events":["user.login"]}`))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestCreate_invalidOrgID_returns400(t *testing.T) {
	h := newTestHandler()
	mux := chi.NewRouter()
	mux.Post("/auth/orgs/{orgID}/webhooks", func(w http.ResponseWriter, r *http.Request) {
		claims := &authclaims.Claims{}
		claims.Subject = "550e8400-e29b-41d4-a716-446655440000"
		r = r.WithContext(authclaims.NewContext(r.Context(), claims))
		h.Create(w, r)
	})

	r := httptest.NewRequest(http.MethodPost, "/auth/orgs/not-a-uuid/webhooks",
		bytes.NewBufferString(`{"url":"https://example.com/hook","events":["user.login"]}`))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestList_invalidOrgID_returns400(t *testing.T) {
	h := newTestHandler()
	mux := chi.NewRouter()
	mux.Get("/auth/orgs/{orgID}/webhooks", h.List)

	r := httptest.NewRequest(http.MethodGet, "/auth/orgs/bad/webhooks", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestListDeliveries_invalidID_returns400(t *testing.T) {
	h := newTestHandler()
	mux := chi.NewRouter()
	mux.Get("/auth/orgs/{orgID}/webhooks/{whID}/deliveries", h.ListDeliveries)

	r := httptest.NewRequest(http.MethodGet,
		"/auth/orgs/550e8400-e29b-41d4-a716-446655440000/webhooks/bad/deliveries", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestDelete_noClaims_returns401(t *testing.T) {
	h := newTestHandler()
	mux := chi.NewRouter()
	mux.Delete("/auth/orgs/{orgID}/webhooks/{whID}", h.Delete)

	r := httptest.NewRequest(http.MethodDelete,
		"/auth/orgs/550e8400-e29b-41d4-a716-446655440000/webhooks/550e8400-e29b-41d4-a716-446655440001", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestGenerateSecret_format(t *testing.T) {
	s, err := generateSecret()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(s) < 10 {
		t.Errorf("secret too short: %s", s)
	}
	if s[:6] != "whsec_" {
		t.Errorf("expected whsec_ prefix, got %s", s[:6])
	}
}

func TestGenerateSecret_uniqueness(t *testing.T) {
	s1, _ := generateSecret()
	s2, _ := generateSecret()
	if s1 == s2 {
		t.Error("expected unique secrets")
	}
}

func TestResponseIsJSON(t *testing.T) {
	h := newTestHandler()
	mux := chi.NewRouter()
	mux.Get("/auth/orgs/{orgID}/webhooks", h.List)

	r := httptest.NewRequest(http.MethodGet, "/auth/orgs/bad/webhooks", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected application/json, got %q", ct)
	}
	_, err := httputil.DecodeJSON[httputil.ErrorResponse](w.Body)
	if err != nil {
		t.Errorf("not valid JSON: %v", err)
	}
}
