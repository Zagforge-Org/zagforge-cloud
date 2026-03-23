//go:build integration

package integration_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func validUploadPayload(orgSlug, repoFullName string) []byte {
	payload := map[string]any{
		"org_slug":       orgSlug,
		"repo_full_name": repoFullName,
		"commit_sha":     fmt.Sprintf("abc%d", time.Now().UnixNano()),
		"branch":         "main",
		"metadata_snapshot": map[string]any{
			"snapshot_version": 2,
			"zigzag_version":   "0.12.0",
			"commit_sha":       fmt.Sprintf("abc%d", time.Now().UnixNano()),
			"branch":           "main",
			"file_tree": []map[string]any{
				{"path": "cmd/main.go", "language": "go", "lines": 42, "sha": "blobsha1"},
				{"path": "README.md", "language": "markdown", "lines": 10, "sha": "blobsha2"},
			},
		},
	}
	b, _ := json.Marshal(payload)
	return b
}

func TestUpload_ValidPayload_Returns201(t *testing.T) {
	env := newTestEnv(t)
	orgSlug := fmt.Sprintf("upload-org-%d", time.Now().UnixNano())
	repoName := fmt.Sprintf("upload-org/repo-%d", time.Now().UnixNano())
	env.seedWithNames(t, orgSlug, repoName)

	body := validUploadPayload(orgSlug, repoName)
	resp := env.postJSON(t, "/api/v1/upload", body, map[string]string{
		"Authorization": "Bearer " + testCLIAPIKey,
	})

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", resp.StatusCode, readBody(t, resp))
	}

	var result struct {
		SnapshotID string `json:"snapshot_id"`
		CreatedAt  string `json:"created_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if result.SnapshotID == "" {
		t.Error("expected non-empty snapshot_id")
	}
}

func TestUpload_NoAuth_Returns401(t *testing.T) {
	env := newTestEnv(t)

	body := validUploadPayload("any-org", "any-org/any-repo")
	resp := env.postJSON(t, "/api/v1/upload", body, nil)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestUpload_WrongKey_Returns401(t *testing.T) {
	env := newTestEnv(t)

	body := validUploadPayload("any-org", "any-org/any-repo")
	resp := env.postJSON(t, "/api/v1/upload", body, map[string]string{
		"Authorization": "Bearer wrong-key",
	})

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestUpload_InvalidJSON_Returns400(t *testing.T) {
	env := newTestEnv(t)

	resp := env.postJSON(t, "/api/v1/upload", []byte("{bad"), map[string]string{
		"Authorization": "Bearer " + testCLIAPIKey,
	})

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestUpload_MissingFields_Returns400(t *testing.T) {
	env := newTestEnv(t)

	body, _ := json.Marshal(map[string]string{"org_slug": "test"})
	resp := env.postJSON(t, "/api/v1/upload", body, map[string]string{
		"Authorization": "Bearer " + testCLIAPIKey,
	})

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestUpload_UnknownOrg_Returns404(t *testing.T) {
	env := newTestEnv(t)

	body := validUploadPayload("nonexistent-org", "nonexistent-org/repo")
	resp := env.postJSON(t, "/api/v1/upload", body, map[string]string{
		"Authorization": "Bearer " + testCLIAPIKey,
	})

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestUpload_UnknownRepo_Returns404(t *testing.T) {
	env := newTestEnv(t)
	orgSlug := fmt.Sprintf("upload-org-%d", time.Now().UnixNano())
	env.seedWithNames(t, orgSlug, "some-org/some-repo")

	body := validUploadPayload(orgSlug, "some-org/nonexistent-repo")
	resp := env.postJSON(t, "/api/v1/upload", body, map[string]string{
		"Authorization": "Bearer " + testCLIAPIKey,
	})

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestUpload_SnapshotVersion1_Returns400(t *testing.T) {
	env := newTestEnv(t)
	orgSlug := fmt.Sprintf("upload-org-%d", time.Now().UnixNano())
	repoName := fmt.Sprintf("upload-org/repo-%d", time.Now().UnixNano())
	env.seedWithNames(t, orgSlug, repoName)

	payload := map[string]any{
		"org_slug":       orgSlug,
		"repo_full_name": repoName,
		"commit_sha":     "abc1234567",
		"branch":         "main",
		"metadata_snapshot": map[string]any{
			"snapshot_version": 1,
			"zigzag_version":   "0.11.0",
			"commit_sha":       "abc1234567",
			"branch":           "main",
			"file_tree":        []map[string]any{{"path": "a.go", "sha": "s1"}},
		},
	}
	body, _ := json.Marshal(payload)
	resp := env.postJSON(t, "/api/v1/upload", body, map[string]string{
		"Authorization": "Bearer " + testCLIAPIKey,
	})

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", resp.StatusCode, readBody(t, resp))
	}
}
