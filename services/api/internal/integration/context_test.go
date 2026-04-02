//go:build integration

package integration_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/LegationPro/zagforge/shared/go/store"
)

func TestContextURL_Head_ValidToken_Returns200(t *testing.T) {
	env := newTestEnv(t)
	orgSlug := fmt.Sprintf("ctx-org-%d", time.Now().UnixNano())
	repoName := fmt.Sprintf("ctx-org/repo-%d", time.Now().UnixNano())
	env.seedWithNames(t, orgSlug, repoName)

	// Upload a snapshot first.
	body := validUploadPayload(orgSlug, repoName)
	uploadResp := env.postJSON(t, "/api/v1/upload", body, map[string]string{
		"Authorization": "Bearer " + testCLIAPIKey,
	})
	if uploadResp.StatusCode != http.StatusCreated {
		t.Fatalf("upload: expected 201, got %d: %s", uploadResp.StatusCode, readBody(t, uploadResp))
	}
	var uploadResult struct {
		SnapshotID string `json:"snapshot_id"`
	}
	json.NewDecoder(uploadResp.Body).Decode(&uploadResult)

	// Create a context token pointing to the snapshot.
	rawToken := fmt.Sprintf("zf_ctx_test_%d", time.Now().UnixNano())
	hash := sha256Hash(rawToken)
	snapUUID, _ := parseUUID(uploadResult.SnapshotID)
	org, _ := env.db.Queries.GetOrganizationBySlug(context.Background(), orgSlug)
	repo, _ := env.db.Queries.GetRepoByFullNameAndOrg(context.Background(), store.GetRepoByFullNameAndOrgParams{
		FullName: repoName, OrgID: org.ID,
	})

	_, err := env.db.Queries.InsertContextToken(context.Background(), store.InsertContextTokenParams{
		RepoID:           repo.ID,
		OrgID:            org.ID,
		TargetSnapshotID: snapUUID,
		TokenHash:        hash,
		Label:            pgtype.Text{String: "test-token", Valid: true},
	})
	if err != nil {
		t.Fatalf("insert context token: %v", err)
	}

	// HEAD the context URL.
	resp := env.head(t, "/v1/context/"+rawToken)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if resp.Header.Get("X-Snapshot-ID") == "" {
		t.Error("expected X-Snapshot-ID header")
	}
	if resp.Header.Get("X-Commit-SHA") == "" {
		t.Error("expected X-Commit-SHA header")
	}
	if resp.Header.Get("Content-Type") != "text/markdown" {
		t.Errorf("expected text/markdown, got %q", resp.Header.Get("Content-Type"))
	}
}

func TestContextURL_Head_InvalidToken_Returns404(t *testing.T) {
	env := newTestEnv(t)

	resp := env.head(t, "/v1/context/nonexistent-token")

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestContextURL_Head_ExpiredToken_Returns410(t *testing.T) {
	env := newTestEnv(t)
	orgSlug := fmt.Sprintf("ctx-org-%d", time.Now().UnixNano())
	repoName := fmt.Sprintf("ctx-org/repo-%d", time.Now().UnixNano())
	env.seedWithNames(t, orgSlug, repoName)

	body := validUploadPayload(orgSlug, repoName)
	uploadResp := env.postJSON(t, "/api/v1/upload", body, map[string]string{
		"Authorization": "Bearer " + testCLIAPIKey,
	})
	if uploadResp.StatusCode != http.StatusCreated {
		t.Fatalf("upload: expected 201, got %d", uploadResp.StatusCode)
	}
	var uploadResult struct {
		SnapshotID string `json:"snapshot_id"`
	}
	json.NewDecoder(uploadResp.Body).Decode(&uploadResult)

	rawToken := fmt.Sprintf("zf_ctx_expired_%d", time.Now().UnixNano())
	hash := sha256Hash(rawToken)
	snapUUID, _ := parseUUID(uploadResult.SnapshotID)
	org, _ := env.db.Queries.GetOrganizationBySlug(context.Background(), orgSlug)
	repo, _ := env.db.Queries.GetRepoByFullNameAndOrg(context.Background(), store.GetRepoByFullNameAndOrgParams{
		FullName: repoName, OrgID: org.ID,
	})

	// Insert token that expired 1 hour ago.
	_, err := env.db.Queries.InsertContextToken(context.Background(), store.InsertContextTokenParams{
		RepoID:           repo.ID,
		OrgID:            org.ID,
		TargetSnapshotID: snapUUID,
		TokenHash:        hash,
		Label:            pgtype.Text{String: "expired", Valid: true},
		ExpiresAt:        pgtype.Timestamptz{Time: time.Now().Add(-1 * time.Hour), Valid: true},
	})
	if err != nil {
		t.Fatalf("insert context token: %v", err)
	}

	resp := env.head(t, "/v1/context/"+rawToken)

	if resp.StatusCode != http.StatusGone {
		t.Fatalf("expected 410, got %d", resp.StatusCode)
	}
}

func sha256Hash(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func parseUUID(s string) (pgtype.UUID, error) {
	var id pgtype.UUID
	err := id.Scan(s)
	return id, err
}
