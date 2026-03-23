package clitoken_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LegationPro/zagforge/api/internal/middleware/clitoken"
)

const testKey = "zf_pk_test123"

func handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func TestAuth_ValidKey(t *testing.T) {
	mw := clitoken.Auth(testKey)
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Authorization", "Bearer "+testKey)
	w := httptest.NewRecorder()

	mw(handler()).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("got %d, want 200", w.Code)
	}
}

func TestAuth_MissingHeader(t *testing.T) {
	mw := clitoken.Auth(testKey)
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	w := httptest.NewRecorder()

	mw(handler()).ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("got %d, want 401", w.Code)
	}
}

func TestAuth_EmptyBearer(t *testing.T) {
	mw := clitoken.Auth(testKey)
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Authorization", "Bearer ")
	w := httptest.NewRecorder()

	mw(handler()).ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("got %d, want 401", w.Code)
	}
}

func TestAuth_WrongKey(t *testing.T) {
	mw := clitoken.Auth(testKey)
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Authorization", "Bearer wrong-key")
	w := httptest.NewRecorder()

	mw(handler()).ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("got %d, want 401", w.Code)
	}
}

func TestAuth_NoBearerPrefix(t *testing.T) {
	mw := clitoken.Auth(testKey)
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Authorization", testKey)
	w := httptest.NewRecorder()

	mw(handler()).ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("got %d, want 401", w.Code)
	}
}

func TestAuth_DoesNotCallNextOnFailure(t *testing.T) {
	mw := clitoken.Auth(testKey)
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	w := httptest.NewRecorder()

	mw(next).ServeHTTP(w, req)

	if called {
		t.Fatal("next handler should not be called on auth failure")
	}
}
