package auth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"

	authmw "github.com/LegationPro/zagforge/auth/internal/middleware/auth"
	"github.com/LegationPro/zagforge/auth/internal/testutil"
	"github.com/LegationPro/zagforge/shared/go/authclaims"
	"github.com/LegationPro/zagforge/shared/go/httputil"
)

func TestAuth_missingToken_returns401(t *testing.T) {
	kp := testutil.GenerateKeyPair(t)
	mw := authmw.Auth(kp.PublicKey, "test-issuer", zap.NewNop())
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}

	body, err := httputil.DecodeJSON[httputil.ErrorResponse](w.Body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Error == nil || *body.Error != authmw.ErrMissingToken.Error() {
		t.Errorf("expected %q, got %v", authmw.ErrMissingToken, body.Error)
	}
}

func TestAuth_invalidToken_returns401(t *testing.T) {
	kp := testutil.GenerateKeyPair(t)
	mw := authmw.Auth(kp.PublicKey, "test-issuer", zap.NewNop())
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.Header.Set("Authorization", "Bearer garbage-token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestAuth_missingBearerPrefix_returns401(t *testing.T) {
	kp := testutil.GenerateKeyPair(t)
	mw := authmw.Auth(kp.PublicKey, "test-issuer", zap.NewNop())
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.Header.Set("Authorization", "Token some-value")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestAuth_validToken_passesThrough(t *testing.T) {
	kp := testutil.GenerateKeyPair(t)

	claims := &authclaims.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "test-issuer",
			Subject:   "user-123",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		Email: "test@example.com",
		Name:  "Test User",
	}

	tokenStr, err := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims).SignedString(kp.PrivateKey)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	var extractedClaims *authclaims.Claims
	mw := authmw.Auth(kp.PublicKey, "test-issuer", zap.NewNop())
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := authclaims.FromContext(r.Context())
		if err != nil {
			t.Errorf("claims not in context: %v", err)
		}
		extractedClaims = c
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.Header.Set("Authorization", "Bearer "+tokenStr)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if extractedClaims == nil {
		t.Fatal("expected claims in context")
	}
	if extractedClaims.Subject != "user-123" {
		t.Errorf("expected subject %q, got %q", "user-123", extractedClaims.Subject)
	}
	if extractedClaims.Email != "test@example.com" {
		t.Errorf("expected email %q, got %q", "test@example.com", extractedClaims.Email)
	}
}

func TestAuth_expiredToken_returns401(t *testing.T) {
	kp := testutil.GenerateKeyPair(t)

	claims := &authclaims.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "test-issuer",
			Subject:   "user-123",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		},
	}

	tokenStr, _ := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims).SignedString(kp.PrivateKey)

	mw := authmw.Auth(kp.PublicKey, "test-issuer", zap.NewNop())
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.Header.Set("Authorization", "Bearer "+tokenStr)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestAuth_wrongIssuer_returns401(t *testing.T) {
	kp := testutil.GenerateKeyPair(t)

	claims := &authclaims.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "wrong-issuer",
			Subject:   "user-123",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	tokenStr, _ := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims).SignedString(kp.PrivateKey)

	mw := authmw.Auth(kp.PublicKey, "test-issuer", zap.NewNop())
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.Header.Set("Authorization", "Bearer "+tokenStr)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestAuth_wrongKey_returns401(t *testing.T) {
	signingKP := testutil.GenerateKeyPair(t)
	verifyKP := testutil.GenerateKeyPair(t) // different key

	claims := &authclaims.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "test-issuer",
			Subject:   "user-123",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	tokenStr, _ := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims).SignedString(signingKP.PrivateKey)

	mw := authmw.Auth(verifyKP.PublicKey, "test-issuer", zap.NewNop())
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.Header.Set("Authorization", "Bearer "+tokenStr)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestAuth_responseIsJSON(t *testing.T) {
	kp := testutil.GenerateKeyPair(t)
	mw := authmw.Auth(kp.PublicKey, "test-issuer", zap.NewNop())
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}
}

func TestUserIDFromContext_noClaims(t *testing.T) {
	_, err := authmw.UserIDFromContext(context.Background())
	if err == nil {
		t.Fatal("expected error with no claims in context")
	}
}
