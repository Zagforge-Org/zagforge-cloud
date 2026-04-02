package org

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

func chiRequest(t *testing.T, method, pattern, target string, h http.HandlerFunc, body string) *httptest.ResponseRecorder {
	t.Helper()
	mux := chi.NewRouter()
	switch method {
	case http.MethodGet:
		mux.Get(pattern, h)
	case http.MethodPut:
		mux.Put(pattern, h)
	case http.MethodDelete:
		mux.Delete(pattern, h)
	case http.MethodPost:
		mux.Post(pattern, h)
	}

	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, target, bytes.NewBufferString(body))
	} else {
		r = httptest.NewRequest(method, target, nil)
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	return w
}

func TestCreate_noClaims_returns401(t *testing.T) {
	h := newTestHandler()
	body := `{"slug":"acme","name":"Acme Corp"}`
	r := httptest.NewRequest(http.MethodPost, "/auth/orgs", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	h.Create(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestCreate_invalidJSON_returns400(t *testing.T) {
	h := newTestHandler()
	r := httptest.NewRequest(http.MethodPost, "/auth/orgs", bytes.NewBufferString("not json"))
	r = requestWithClaims(r, "550e8400-e29b-41d4-a716-446655440000")
	w := httptest.NewRecorder()

	h.Create(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCreate_missingSlug_returns400(t *testing.T) {
	h := newTestHandler()
	body := `{"name":"Acme Corp"}`
	r := httptest.NewRequest(http.MethodPost, "/auth/orgs", bytes.NewBufferString(body))
	r = requestWithClaims(r, "550e8400-e29b-41d4-a716-446655440000")
	w := httptest.NewRecorder()

	h.Create(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCreate_slugTooShort_returns400(t *testing.T) {
	h := newTestHandler()
	body := `{"slug":"a","name":"Acme"}`
	r := httptest.NewRequest(http.MethodPost, "/auth/orgs", bytes.NewBufferString(body))
	r = requestWithClaims(r, "550e8400-e29b-41d4-a716-446655440000")
	w := httptest.NewRecorder()

	h.Create(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestGet_invalidOrgID_returns400(t *testing.T) {
	h := newTestHandler()

	w := chiRequest(t, http.MethodGet, "/auth/orgs/{orgID}", "/auth/orgs/not-a-uuid", h.Get, "")

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestList_noClaims_returns401(t *testing.T) {
	h := newTestHandler()
	r := httptest.NewRequest(http.MethodGet, "/auth/orgs", nil)
	w := httptest.NewRecorder()

	h.List(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestUpdate_noClaims_returns401(t *testing.T) {
	h := newTestHandler()

	mux := chi.NewRouter()
	mux.Put("/auth/orgs/{orgID}", h.Update)

	r := httptest.NewRequest(http.MethodPut, "/auth/orgs/550e8400-e29b-41d4-a716-446655440000", bytes.NewBufferString(`{"name":"test"}`))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestDelete_noClaims_returns401(t *testing.T) {
	h := newTestHandler()

	mux := chi.NewRouter()
	mux.Delete("/auth/orgs/{orgID}", h.Delete)

	r := httptest.NewRequest(http.MethodDelete, "/auth/orgs/550e8400-e29b-41d4-a716-446655440000", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestCreate_missingName_returns400(t *testing.T) {
	h := newTestHandler()
	body := `{"slug":"acme"}`
	r := httptest.NewRequest(http.MethodPost, "/auth/orgs", bytes.NewBufferString(body))
	r = requestWithClaims(r, "550e8400-e29b-41d4-a716-446655440000")
	w := httptest.NewRecorder()

	h.Create(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCreate_slugTooLong_returns400(t *testing.T) {
	h := newTestHandler()
	longSlug := string(make([]byte, 51))
	for i := range longSlug {
		longSlug = longSlug[:i] + "a" + longSlug[i+1:]
	}
	body := `{"slug":"` + longSlug + `","name":"Test"}`
	r := httptest.NewRequest(http.MethodPost, "/auth/orgs", bytes.NewBufferString(body))
	r = requestWithClaims(r, "550e8400-e29b-41d4-a716-446655440000")
	w := httptest.NewRecorder()

	h.Create(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCreate_emptyBody_returns400(t *testing.T) {
	h := newTestHandler()
	r := httptest.NewRequest(http.MethodPost, "/auth/orgs", bytes.NewBufferString("{}"))
	r = requestWithClaims(r, "550e8400-e29b-41d4-a716-446655440000")
	w := httptest.NewRecorder()

	h.Create(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCreate_invalidSubjectUUID_returns401(t *testing.T) {
	h := newTestHandler()
	body := `{"slug":"acme","name":"Acme"}`
	r := httptest.NewRequest(http.MethodPost, "/auth/orgs", bytes.NewBufferString(body))
	claims := &authclaims.Claims{}
	claims.Subject = "not-a-uuid"
	r = r.WithContext(authclaims.NewContext(r.Context(), claims))
	w := httptest.NewRecorder()

	h.Create(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestListMembers_invalidOrgID_returns400(t *testing.T) {
	h := newTestHandler()
	w := chiRequest(t, http.MethodGet, "/auth/orgs/{orgID}/members",
		"/auth/orgs/not-a-uuid/members", h.ListMembers, "")

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestResponseIsJSON(t *testing.T) {
	h := newTestHandler()
	r := httptest.NewRequest(http.MethodPost, "/auth/orgs", bytes.NewBufferString("bad"))
	r = requestWithClaims(r, "550e8400-e29b-41d4-a716-446655440000")
	w := httptest.NewRecorder()

	h.Create(w, r)

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}

	_, err := httputil.DecodeJSON[httputil.ErrorResponse](w.Body)
	if err != nil {
		t.Errorf("response is not valid JSON: %v", err)
	}
}
