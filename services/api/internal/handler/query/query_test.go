package query_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/LegationPro/zagforge/api/internal/handler/query"
	"go.uber.org/zap"
)

func TestQuery_InvalidJSON(t *testing.T) {
	h := query.NewHandler(nil, nil, nil, nil, nil, zap.NewNop())
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString("{bad"))
	w := httptest.NewRecorder()

	h.Query(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", w.Code)
	}
}

func TestQuery_EmptyBody(t *testing.T) {
	h := query.NewHandler(nil, nil, nil, nil, nil, zap.NewNop())
	body, _ := json.Marshal(map[string]string{})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.Query(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", w.Code)
	}
}

func TestQuery_MissingQuestion(t *testing.T) {
	h := query.NewHandler(nil, nil, nil, nil, nil, zap.NewNop())
	body, _ := json.Marshal(map[string]string{"snapshot_id": "some-uuid"})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.Query(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", w.Code)
	}
}

func TestQuery_QuestionTooLong(t *testing.T) {
	h := query.NewHandler(nil, nil, nil, nil, nil, zap.NewNop())
	long := strings.Repeat("a", 4001)
	body, _ := json.Marshal(map[string]string{"question": long})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.Query(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", w.Code)
	}
}

func TestQuery_InvalidSnapshotID(t *testing.T) {
	h := query.NewHandler(nil, nil, nil, nil, nil, zap.NewNop())
	body, _ := json.Marshal(map[string]string{
		"question":    "what does main do?",
		"snapshot_id": "not-a-uuid",
	})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.Query(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", w.Code)
	}
}

// Note: auth is enforced by OrgScope middleware (not the handler).
// A valid payload with no auth context would never reach Query() in production.
// Cross-org access is tested via integration tests.

func TestConfig_SystemPromptNotEmpty(t *testing.T) {
	if query.SystemPrompt == "" {
		t.Error("SystemPrompt should not be empty")
	}
}

func TestConfig_ProviderOrderNotEmpty(t *testing.T) {
	if len(query.ProviderOrder) == 0 {
		t.Error("ProviderOrder should not be empty")
	}
}

func TestConfig_ProviderOrderContainsExpected(t *testing.T) {
	expected := map[string]bool{"anthropic": false, "openai": false}
	for _, p := range query.ProviderOrder {
		expected[p] = true
	}
	for name, found := range expected {
		if !found {
			t.Errorf("ProviderOrder missing %q", name)
		}
	}
}
