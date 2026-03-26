package team

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/LegationPro/zagforge/auth/internal/db"
	"github.com/LegationPro/zagforge/auth/internal/service/audit"
	"github.com/LegationPro/zagforge/shared/go/authclaims"
	"github.com/LegationPro/zagforge/shared/go/httputil"
)

func newTestHandler() *Handler {
	return NewHandler(&db.DB{}, audit.New(nil), zap.NewNop())
}

func requestWithClaims(r *http.Request, sub string) *http.Request {
	claims := &authclaims.Claims{}
	claims.Subject = sub
	return r.WithContext(authclaims.NewContext(r.Context(), claims))
}

func TestCreate_noClaims_returns401(t *testing.T) {
	h := newTestHandler()
	mux := chi.NewRouter()
	mux.Post("/auth/orgs/{orgID}/teams", h.Create)

	r := httptest.NewRequest(http.MethodPost, "/auth/orgs/550e8400-e29b-41d4-a716-446655440000/teams",
		bytes.NewBufferString(`{"slug":"eng","name":"Engineering"}`))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestCreate_invalidOrgID_returns400(t *testing.T) {
	h := newTestHandler()
	mux := chi.NewRouter()
	mux.Post("/auth/orgs/{orgID}/teams", func(w http.ResponseWriter, r *http.Request) {
		r = requestWithClaims(r, "550e8400-e29b-41d4-a716-446655440000")
		h.Create(w, r)
	})

	r := httptest.NewRequest(http.MethodPost, "/auth/orgs/not-a-uuid/teams",
		bytes.NewBufferString(`{"slug":"eng","name":"Engineering"}`))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestGet_invalidTeamID_returns400(t *testing.T) {
	h := newTestHandler()
	mux := chi.NewRouter()
	mux.Get("/auth/orgs/{orgID}/teams/{teamID}", h.Get)

	r := httptest.NewRequest(http.MethodGet,
		"/auth/orgs/550e8400-e29b-41d4-a716-446655440000/teams/not-a-uuid", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestList_invalidOrgID_returns400(t *testing.T) {
	h := newTestHandler()
	mux := chi.NewRouter()
	mux.Get("/auth/orgs/{orgID}/teams", h.List)

	r := httptest.NewRequest(http.MethodGet, "/auth/orgs/not-a-uuid/teams", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestAddMember_noClaims_returns401(t *testing.T) {
	h := newTestHandler()
	mux := chi.NewRouter()
	mux.Post("/auth/orgs/{orgID}/teams/{teamID}/members", h.AddMember)

	r := httptest.NewRequest(http.MethodPost,
		"/auth/orgs/550e8400-e29b-41d4-a716-446655440000/teams/550e8400-e29b-41d4-a716-446655440001/members",
		bytes.NewBufferString(`{"user_id":"550e8400-e29b-41d4-a716-446655440002","role":"member"}`))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestResponseIsJSON(t *testing.T) {
	h := newTestHandler()
	mux := chi.NewRouter()
	mux.Get("/auth/orgs/{orgID}/teams/{teamID}", h.Get)

	r := httptest.NewRequest(http.MethodGet,
		"/auth/orgs/550e8400-e29b-41d4-a716-446655440000/teams/not-a-uuid", nil)
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
