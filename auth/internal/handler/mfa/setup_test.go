package mfa

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"

	"github.com/LegationPro/zagforge/auth/internal/db"
	"github.com/LegationPro/zagforge/auth/internal/service/audit"
	"github.com/LegationPro/zagforge/shared/go/authclaims"
	"github.com/LegationPro/zagforge/shared/go/httputil"
)

func newTestHandler() *Handler {
	return NewHandler(&db.DB{}, nil, nil, nil, audit.New(nil), zap.NewNop())
}

func requestWithClaims(r *http.Request, sub, email string) *http.Request {
	claims := &authclaims.Claims{Email: email}
	claims.Subject = sub
	return r.WithContext(authclaims.NewContext(r.Context(), claims))
}

func TestSetup_noClaims_returns401(t *testing.T) {
	h := newTestHandler()
	r := httptest.NewRequest(http.MethodPost, "/auth/mfa/totp/setup", nil)
	w := httptest.NewRecorder()

	h.Setup(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestVerify_noClaims_returns401(t *testing.T) {
	h := newTestHandler()
	r := httptest.NewRequest(http.MethodPost, "/auth/mfa/totp/verify", bytes.NewBufferString(`{"code":"123456"}`))
	w := httptest.NewRecorder()

	h.Verify(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestVerify_invalidJSON_returns400(t *testing.T) {
	h := newTestHandler()
	r := httptest.NewRequest(http.MethodPost, "/auth/mfa/totp/verify", bytes.NewBufferString("not json"))
	r = requestWithClaims(r, "550e8400-e29b-41d4-a716-446655440000", "user@example.com")
	w := httptest.NewRecorder()

	h.Verify(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestVerify_missingCode_returns400(t *testing.T) {
	h := newTestHandler()
	r := httptest.NewRequest(http.MethodPost, "/auth/mfa/totp/verify", bytes.NewBufferString(`{}`))
	r = requestWithClaims(r, "550e8400-e29b-41d4-a716-446655440000", "user@example.com")
	w := httptest.NewRecorder()

	h.Verify(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestVerify_wrongCodeLength_returns400(t *testing.T) {
	h := newTestHandler()
	r := httptest.NewRequest(http.MethodPost, "/auth/mfa/totp/verify", bytes.NewBufferString(`{"code":"12345"}`))
	r = requestWithClaims(r, "550e8400-e29b-41d4-a716-446655440000", "user@example.com")
	w := httptest.NewRecorder()

	h.Verify(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestDisable_noClaims_returns401(t *testing.T) {
	h := newTestHandler()
	r := httptest.NewRequest(http.MethodPost, "/auth/mfa/totp/disable", nil)
	w := httptest.NewRecorder()

	h.Disable(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestChallenge_invalidJSON_returns400(t *testing.T) {
	h := newTestHandler()
	r := httptest.NewRequest(http.MethodPost, "/auth/mfa/totp/challenge", bytes.NewBufferString("bad"))
	w := httptest.NewRecorder()

	h.Challenge(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestChallenge_missingFields_returns400(t *testing.T) {
	h := newTestHandler()
	r := httptest.NewRequest(http.MethodPost, "/auth/mfa/totp/challenge", bytes.NewBufferString(`{}`))
	w := httptest.NewRecorder()

	h.Challenge(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestBackupVerify_invalidJSON_returns400(t *testing.T) {
	h := newTestHandler()
	r := httptest.NewRequest(http.MethodPost, "/auth/mfa/backup-codes/verify", bytes.NewBufferString("bad"))
	w := httptest.NewRecorder()

	h.BackupCodeVerify(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestResponseIsJSON(t *testing.T) {
	h := newTestHandler()
	r := httptest.NewRequest(http.MethodPost, "/auth/mfa/totp/verify", bytes.NewBufferString("bad"))
	r = requestWithClaims(r, "550e8400-e29b-41d4-a716-446655440000", "user@example.com")
	w := httptest.NewRecorder()

	h.Verify(w, r)

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}

	_, err := httputil.DecodeJSON[httputil.ErrorResponse](w.Body)
	if err != nil {
		t.Errorf("response is not valid JSON: %v", err)
	}
}
