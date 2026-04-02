package aikeys

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/LegationPro/zagforge/api/internal/service/encryption"
	"go.uber.org/zap"
)

func newTestHandler(t *testing.T) *Handler {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	enc, err := encryption.New(key)
	if err != nil {
		t.Fatalf("encryption.New: %v", err)
	}
	// nil DB — tests will only exercise validation paths
	return NewHandler(nil, enc, zap.NewNop())
}

func TestUpsert_InvalidJSON(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodPut, "/", bytes.NewBufferString("{bad"))
	w := httptest.NewRecorder()

	h.Upsert(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", w.Code)
	}
}

func TestUpsert_EmptyBody(t *testing.T) {
	h := newTestHandler(t)
	body, _ := json.Marshal(map[string]string{})
	req := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.Upsert(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", w.Code)
	}
}

func TestUpsert_MissingProvider(t *testing.T) {
	h := newTestHandler(t)
	body, _ := json.Marshal(map[string]string{"raw_key": "sk-ant-supersecretkey"})
	req := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.Upsert(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", w.Code)
	}
}

func TestUpsert_MissingRawKey(t *testing.T) {
	h := newTestHandler(t)
	body, _ := json.Marshal(map[string]string{"provider": "anthropic"})
	req := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.Upsert(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", w.Code)
	}
}

func TestUpsert_KeyTooShort(t *testing.T) {
	h := newTestHandler(t)
	body, _ := json.Marshal(map[string]string{"provider": "openai", "raw_key": "short"})
	req := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.Upsert(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", w.Code)
	}
	if !strings.Contains(w.Body.String(), "at least 8") {
		t.Errorf("expected key too short error, got: %s", w.Body.String())
	}
}

// Note: auth is enforced by OrgScope middleware, not the handler.
// Cross-org access is tested via integration tests.

func TestHintGeneration(t *testing.T) {
	tests := []struct {
		name   string
		rawKey string
		want   string
	}{
		{"long key", "sk-ant-api01-supersecretkey", "...tkey"},
		{"exactly 8 chars", "12345678", "...5678"},
		{"16 chars", "abcdefghijklmnop", "...mnop"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := "..." + tt.rawKey[len(tt.rawKey)-4:]
			if got != tt.want {
				t.Errorf("hint = %q, want %q", got, tt.want)
			}
		})
	}
}
