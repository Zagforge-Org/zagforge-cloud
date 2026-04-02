package auth_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"

	"github.com/LegationPro/zagforge/api/internal/middleware/auth"
	"github.com/LegationPro/zagforge/shared/go/authclaims"
	"github.com/LegationPro/zagforge/shared/go/httputil"
)

func testPubKey(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return pub, priv
}

func TestAuth_missingToken_returns401(t *testing.T) {
	pub, _ := testPubKey(t)
	mw := auth.Auth(pub, "test-issuer", zap.NewNop())
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}

	body, err := httputil.DecodeJSON[httputil.ErrorResponse](w.Body)
	if err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if body.Error == nil || *body.Error != auth.ErrMissingToken.Error() {
		t.Errorf("expected %q, got %v", auth.ErrMissingToken, body.Error)
	}
}

func TestAuth_invalidToken_returns401(t *testing.T) {
	pub, _ := testPubKey(t)
	mw := auth.Auth(pub, "test-issuer", zap.NewNop())
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	r.Header.Set("Authorization", "Bearer invalid-token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}

	body, err := httputil.DecodeJSON[httputil.ErrorResponse](w.Body)
	if err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if body.Error == nil || *body.Error != auth.ErrInvalidToken.Error() {
		t.Errorf("expected %q, got %v", auth.ErrInvalidToken, body.Error)
	}
}

func TestAuth_missingBearerPrefix_returns401(t *testing.T) {
	pub, _ := testPubKey(t)
	mw := auth.Auth(pub, "test-issuer", zap.NewNop())
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	r.Header.Set("Authorization", "Token some-value")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestAuth_validToken_passesThrough(t *testing.T) {
	pub, priv := testPubKey(t)

	claims := &authclaims.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "test-issuer",
			Subject:   "user-123",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		Email: "test@example.com",
	}

	tokenStr, err := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims).SignedString(priv)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	var extracted *authclaims.Claims
	mw := auth.Auth(pub, "test-issuer", zap.NewNop())
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := auth.ClaimsFromContext(r.Context())
		if err != nil {
			t.Errorf("claims not in context: %v", err)
		}
		extracted = c
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	r.Header.Set("Authorization", "Bearer "+tokenStr)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if extracted == nil {
		t.Fatal("expected claims in context")
	}
	if extracted.Subject != "user-123" {
		t.Errorf("expected subject %q, got %q", "user-123", extracted.Subject)
	}
}

func TestAuth_responseIsJSON(t *testing.T) {
	pub, _ := testPubKey(t)
	mw := auth.Auth(pub, "test-issuer", zap.NewNop())
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}
}

func TestClaimsFromContext_noClaims_returnsError(t *testing.T) {
	_, err := auth.ClaimsFromContext(context.Background())
	if err == nil {
		t.Fatal("expected error when no claims in context")
	}
}
