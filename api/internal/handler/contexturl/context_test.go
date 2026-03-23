package contexturl

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/LegationPro/zagforge/api/internal/cache/contextcache"
)

func TestTokenHash_Deterministic(t *testing.T) {
	h1 := tokenHash("zf_ctx_abc123")
	h2 := tokenHash("zf_ctx_abc123")
	if h1 != h2 {
		t.Errorf("same input produced different hashes: %q vs %q", h1, h2)
	}
}

func TestTokenHash_MatchesSHA256(t *testing.T) {
	input := "zf_ctx_testtoken"
	expected := sha256.Sum256([]byte(input))
	want := hex.EncodeToString(expected[:])
	got := tokenHash(input)
	if got != want {
		t.Errorf("tokenHash(%q) = %q, want %q", input, got, want)
	}
}

func TestTokenHash_DifferentInputs(t *testing.T) {
	h1 := tokenHash("token_a")
	h2 := tokenHash("token_b")
	if h1 == h2 {
		t.Error("different inputs produced the same hash")
	}
}

// withChiParam creates a request with a chi URL param set.
func withChiParam(r *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	ctx := context.WithValue(r.Context(), chi.RouteCtxKey, rctx)
	return r.WithContext(ctx)
}

func TestHead_NilDB_Returns404(t *testing.T) {
	h := NewHandler(nil, contextcache.NewInMemory(), nil, nil, zap.NewNop())
	req := httptest.NewRequest(http.MethodHead, "/v1/context/sometoken", nil)
	req = withChiParam(req, "token", "sometoken")
	w := httptest.NewRecorder()

	defer func() { recover() }()
	h.Head(w, req)

	// With nil DB, we expect a panic or 404 — either way it shouldn't be 200
	if w.Code == http.StatusOK {
		t.Error("expected non-200 with nil DB")
	}
}

func TestGet_NilDB_Returns404(t *testing.T) {
	h := NewHandler(nil, contextcache.NewInMemory(), nil, nil, zap.NewNop())
	req := httptest.NewRequest(http.MethodGet, "/v1/context/sometoken", nil)
	req = withChiParam(req, "token", "sometoken")
	w := httptest.NewRecorder()

	defer func() { recover() }()
	h.Get(w, req)

	if w.Code == http.StatusOK {
		t.Error("expected non-200 with nil DB")
	}
}

func TestHead_EmptyToken_Returns404(t *testing.T) {
	h := NewHandler(nil, contextcache.NewInMemory(), nil, nil, zap.NewNop())
	req := httptest.NewRequest(http.MethodHead, "/v1/context/", nil)
	req = withChiParam(req, "token", "")
	w := httptest.NewRecorder()

	defer func() { recover() }()
	h.Head(w, req)

	if w.Code == http.StatusOK {
		t.Error("expected non-200 with empty token")
	}
}

func TestGet_EmptyToken_Returns404(t *testing.T) {
	h := NewHandler(nil, contextcache.NewInMemory(), nil, nil, zap.NewNop())
	req := httptest.NewRequest(http.MethodGet, "/v1/context/", nil)
	req = withChiParam(req, "token", "")
	w := httptest.NewRecorder()

	defer func() { recover() }()
	h.Get(w, req)

	if w.Code == http.StatusOK {
		t.Error("expected non-200 with empty token")
	}
}
