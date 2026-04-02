package upload_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LegationPro/zagforge/api/internal/handler/upload"
	"go.uber.org/zap"
)

func TestUpload_InvalidJSON(t *testing.T) {
	h := upload.NewHandler(nil, nil, zap.NewNop())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/upload", bytes.NewBufferString("{bad json}"))
	w := httptest.NewRecorder()

	h.Upload(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", w.Code)
	}
}

func TestUpload_EmptyBody(t *testing.T) {
	h := upload.NewHandler(nil, nil, zap.NewNop())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/upload", bytes.NewBufferString("{}"))
	w := httptest.NewRecorder()

	h.Upload(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", w.Code)
	}
}

func TestUpload_MissingRequiredFields(t *testing.T) {
	tests := []struct {
		name string
		body map[string]any
	}{
		{
			"missing org_slug",
			map[string]any{
				"repo_full_name": "org/repo",
				"commit_sha":     "abc1234",
				"branch":         "main",
				"metadata_snapshot": map[string]any{
					"snapshot_version": 2,
					"zigzag_version":   "0.12.0",
					"commit_sha":       "abc1234",
					"branch":           "main",
					"file_tree":        []map[string]any{{"path": "a.go", "sha": "sha1"}},
				},
			},
		},
		{
			"missing repo_full_name",
			map[string]any{
				"org_slug":   "my-org",
				"commit_sha": "abc1234",
				"branch":     "main",
				"metadata_snapshot": map[string]any{
					"snapshot_version": 2,
					"zigzag_version":   "0.12.0",
					"commit_sha":       "abc1234",
					"branch":           "main",
					"file_tree":        []map[string]any{{"path": "a.go", "sha": "sha1"}},
				},
			},
		},
		{
			"missing branch",
			map[string]any{
				"org_slug":       "my-org",
				"repo_full_name": "org/repo",
				"commit_sha":     "abc1234",
				"metadata_snapshot": map[string]any{
					"snapshot_version": 2,
					"zigzag_version":   "0.12.0",
					"commit_sha":       "abc1234",
					"branch":           "main",
					"file_tree":        []map[string]any{{"path": "a.go", "sha": "sha1"}},
				},
			},
		},
		{
			"missing commit_sha",
			map[string]any{
				"org_slug":       "my-org",
				"repo_full_name": "org/repo",
				"branch":         "main",
				"metadata_snapshot": map[string]any{
					"snapshot_version": 2,
					"zigzag_version":   "0.12.0",
					"commit_sha":       "abc1234",
					"branch":           "main",
					"file_tree":        []map[string]any{{"path": "a.go", "sha": "sha1"}},
				},
			},
		},
		{
			"missing metadata_snapshot",
			map[string]any{
				"org_slug":       "my-org",
				"repo_full_name": "org/repo",
				"commit_sha":     "abc1234",
				"branch":         "main",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.body)
			h := upload.NewHandler(nil, nil, zap.NewNop())
			req := httptest.NewRequest(http.MethodPost, "/api/v1/upload", bytes.NewReader(body))
			w := httptest.NewRecorder()

			h.Upload(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("got %d, want 400", w.Code)
			}
		})
	}
}

func TestUpload_CommitShaTooShort(t *testing.T) {
	body, _ := json.Marshal(validUploadBody(func(m map[string]any) {
		m["commit_sha"] = "abc"
	}))
	h := upload.NewHandler(nil, nil, zap.NewNop())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/upload", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.Upload(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", w.Code)
	}
}

func TestUpload_SnapshotVersionNot2(t *testing.T) {
	body, _ := json.Marshal(validUploadBody(func(m map[string]any) {
		meta := m["metadata_snapshot"].(map[string]any)
		meta["snapshot_version"] = 1
	}))
	h := upload.NewHandler(nil, nil, zap.NewNop())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/upload", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.Upload(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", w.Code)
	}
}

func TestUpload_MissingFileTreeEntry(t *testing.T) {
	body, _ := json.Marshal(validUploadBody(func(m map[string]any) {
		meta := m["metadata_snapshot"].(map[string]any)
		meta["file_tree"] = []map[string]any{{"path": "a.go"}} // missing sha
	}))
	h := upload.NewHandler(nil, nil, zap.NewNop())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/upload", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.Upload(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", w.Code)
	}
}

func TestUpload_ValidPayloadButNoDB(t *testing.T) {
	body, _ := json.Marshal(validUploadBody(nil))
	h := upload.NewHandler(nil, nil, zap.NewNop())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/upload", bytes.NewReader(body))
	w := httptest.NewRecorder()

	// Passes validation but panics or returns 404/500 on DB call (nil db).
	// We just verify it gets past the validation stage.
	defer func() { recover() }()
	h.Upload(w, req)

	if w.Code == http.StatusBadRequest {
		t.Error("valid payload should not return 400")
	}
}

// validUploadBody returns a valid upload request body. Pass a mutator to override fields.
func validUploadBody(mutate func(map[string]any)) map[string]any {
	m := map[string]any{
		"org_slug":       "my-org",
		"repo_full_name": "my-org/my-repo",
		"commit_sha":     "abc1234deadbeef",
		"branch":         "main",
		"metadata_snapshot": map[string]any{
			"snapshot_version": 2,
			"zigzag_version":   "0.12.0",
			"commit_sha":       "abc1234deadbeef",
			"branch":           "main",
			"file_tree": []map[string]any{
				{"path": "cmd/main.go", "language": "go", "lines": 42, "sha": "blobsha1"},
			},
		},
	}
	if mutate != nil {
		mutate(m)
	}
	return m
}
