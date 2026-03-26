package user

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"

	"github.com/LegationPro/zagforge/auth/internal/db"
	"github.com/LegationPro/zagforge/auth/internal/handler"
	"github.com/LegationPro/zagforge/shared/go/authclaims"
	"github.com/LegationPro/zagforge/shared/go/httputil"
)

func newTestHandler() *Handler {
	return NewHandler(&db.DB{}, zap.NewNop())
}

func requestWithClaims(r *http.Request, sub string) *http.Request {
	claims := &authclaims.Claims{}
	claims.Subject = sub
	return r.WithContext(authclaims.NewContext(r.Context(), claims))
}

func TestGetMe_noClaims_returns401(t *testing.T) {
	h := newTestHandler()
	r := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	w := httptest.NewRecorder()

	h.GetMe(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestUpdateMe_noClaims_returns401(t *testing.T) {
	h := newTestHandler()
	body := `{"first_name":"Jane"}`
	r := httptest.NewRequest(http.MethodPut, "/auth/me", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	h.UpdateMe(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestUpdateMe_invalidJSON_returns400(t *testing.T) {
	h := newTestHandler()
	r := httptest.NewRequest(http.MethodPut, "/auth/me", bytes.NewBufferString("not json"))
	r = requestWithClaims(r, "550e8400-e29b-41d4-a716-446655440000")
	w := httptest.NewRecorder()

	h.UpdateMe(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestUpdateMe_invalidVisibility_returns400(t *testing.T) {
	h := newTestHandler()
	body := `{"first_name":"Jane","profile_visibility":"invalid"}`
	r := httptest.NewRequest(http.MethodPut, "/auth/me", bytes.NewBufferString(body))
	r = requestWithClaims(r, "550e8400-e29b-41d4-a716-446655440000")
	w := httptest.NewRecorder()

	h.UpdateMe(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestUpdateOnboarding_invalidStep_returns400(t *testing.T) {
	h := newTestHandler()
	body := `{"step":"invalid"}`
	r := httptest.NewRequest(http.MethodPut, "/auth/me/onboarding", bytes.NewBufferString(body))
	r = requestWithClaims(r, "550e8400-e29b-41d4-a716-446655440000")
	w := httptest.NewRecorder()

	h.UpdateOnboarding(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestUpdateOnboarding_missingStep_returns400(t *testing.T) {
	h := newTestHandler()
	body := `{}`
	r := httptest.NewRequest(http.MethodPut, "/auth/me/onboarding", bytes.NewBufferString(body))
	r = requestWithClaims(r, "550e8400-e29b-41d4-a716-446655440000")
	w := httptest.NewRecorder()

	h.UpdateOnboarding(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestUpdateOnboarding_noClaims_returns401(t *testing.T) {
	h := newTestHandler()
	r := httptest.NewRequest(http.MethodPut, "/auth/me/onboarding", bytes.NewBufferString(`{"step":"completed"}`))
	w := httptest.NewRecorder()

	h.UpdateOnboarding(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestUserIDFromContext_noClaims(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	_, err := handler.UserIDFromContext(r)
	if err == nil {
		t.Fatal("expected error with no claims")
	}
}

func TestUserIDFromContext_invalidSubject(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	claims := &authclaims.Claims{}
	claims.Subject = "not-a-uuid"
	r = r.WithContext(authclaims.NewContext(context.Background(), claims))

	_, err := handler.UserIDFromContext(r)
	if err == nil {
		t.Fatal("expected error for non-UUID subject")
	}
}

func TestUpdateMe_invalidAge_returns400(t *testing.T) {
	h := newTestHandler()
	body := `{"first_name":"Jane","age":5}`
	r := httptest.NewRequest(http.MethodPut, "/auth/me", bytes.NewBufferString(body))
	r = requestWithClaims(r, "550e8400-e29b-41d4-a716-446655440000")
	w := httptest.NewRecorder()

	h.UpdateMe(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for age < 13, got %d", w.Code)
	}
}

func TestUpdateOnboarding_invalidJSON_returns400(t *testing.T) {
	h := newTestHandler()
	r := httptest.NewRequest(http.MethodPut, "/auth/me/onboarding", bytes.NewBufferString("not json"))
	r = requestWithClaims(r, "550e8400-e29b-41d4-a716-446655440000")
	w := httptest.NewRecorder()

	h.UpdateOnboarding(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestListIdentities_noClaims_returns401(t *testing.T) {
	h := newTestHandler()
	r := httptest.NewRequest(http.MethodGet, "/auth/me/identities", nil)
	w := httptest.NewRecorder()

	h.ListIdentities(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestResponseIsJSON(t *testing.T) {
	h := newTestHandler()
	r := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	w := httptest.NewRecorder()

	h.GetMe(w, r)

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}

	_, err := httputil.DecodeJSON[httputil.ErrorResponse](w.Body)
	if err != nil {
		t.Errorf("response is not valid JSON: %v", err)
	}
}
