package invite

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
	mux.Post("/auth/orgs/{orgID}/invites", h.Create)

	r := httptest.NewRequest(http.MethodPost, "/auth/orgs/550e8400-e29b-41d4-a716-446655440000/invites",
		bytes.NewBufferString(`{"email":"user@example.com","role":"member"}`))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestAccept_noClaims_returns401(t *testing.T) {
	h := newTestHandler()
	r := httptest.NewRequest(http.MethodPost, "/auth/invites/accept", bytes.NewBufferString(`{"token":"some-token"}`))
	w := httptest.NewRecorder()

	h.Accept(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestAccept_missingToken_returns400(t *testing.T) {
	h := newTestHandler()
	r := httptest.NewRequest(http.MethodPost, "/auth/invites/accept", bytes.NewBufferString(`{}`))
	r = requestWithClaims(r, "550e8400-e29b-41d4-a716-446655440000")
	w := httptest.NewRecorder()

	h.Accept(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestAccept_invalidJSON_returns400(t *testing.T) {
	h := newTestHandler()
	r := httptest.NewRequest(http.MethodPost, "/auth/invites/accept", bytes.NewBufferString("not json"))
	r = requestWithClaims(r, "550e8400-e29b-41d4-a716-446655440000")
	w := httptest.NewRecorder()

	h.Accept(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCreate_invalidOrgID_returns400(t *testing.T) {
	h := newTestHandler()

	mux := chi.NewRouter()
	mux.Post("/auth/orgs/{orgID}/invites", func(w http.ResponseWriter, r *http.Request) {
		r = requestWithClaims(r, "550e8400-e29b-41d4-a716-446655440000")
		h.Create(w, r)
	})

	r := httptest.NewRequest(http.MethodPost, "/auth/orgs/not-a-uuid/invites",
		bytes.NewBufferString(`{"email":"user@example.com","role":"member"}`))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestRevoke_noClaims_returns401(t *testing.T) {
	h := newTestHandler()

	mux := chi.NewRouter()
	mux.Delete("/auth/orgs/{orgID}/invites/{inviteID}", h.Revoke)

	r := httptest.NewRequest(http.MethodDelete,
		"/auth/orgs/550e8400-e29b-41d4-a716-446655440000/invites/550e8400-e29b-41d4-a716-446655440001", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestResponseIsJSON(t *testing.T) {
	h := newTestHandler()
	r := httptest.NewRequest(http.MethodPost, "/auth/invites/accept", bytes.NewBufferString("bad json"))
	r = requestWithClaims(r, "550e8400-e29b-41d4-a716-446655440000")
	w := httptest.NewRecorder()

	h.Accept(w, r)

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}

	_, err := httputil.DecodeJSON[httputil.ErrorResponse](w.Body)
	if err != nil {
		t.Errorf("response is not valid JSON: %v", err)
	}
}

func TestGenerateInviteToken_uniqueness(t *testing.T) {
	t1, err := generateInviteToken()
	if err != nil {
		t.Fatalf("generate 1: %v", err)
	}
	t2, err := generateInviteToken()
	if err != nil {
		t.Fatalf("generate 2: %v", err)
	}
	if t1 == t2 {
		t.Error("expected unique tokens")
	}
	if len(t1) != 64 {
		t.Errorf("expected token length 64, got %d", len(t1))
	}
}
