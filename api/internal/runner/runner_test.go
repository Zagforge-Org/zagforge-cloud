package runner_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	github "github.com/LegationPro/zagforge-mvp-impl/shared/go/provider/github"
	"github.com/LegationPro/zagforge-mvp-impl/api/internal/runner"
)

// mockCloner is a test double for runner.RepoCloner.
type mockCloner struct {
	token    string
	tokenErr error
	cloneErr error
}

func (m *mockCloner) GenerateCloneToken(_ context.Context, _ int64) (string, error) {
	return m.token, m.tokenErr
}

func (m *mockCloner) CloneRepo(_ context.Context, _, _, _, dst string) error {
	if m.cloneErr != nil {
		return m.cloneErr
	}
	// Create the destination directory so the runner can proceed.
	return os.MkdirAll(dst, 0o755)
}

// mockZigzag writes a tiny shell script that exits 0, returning its path.
func mockZigzag(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "zigzag")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("failed to write mock zigzag: %v", err)
	}
	return path
}

func newRunner(t *testing.T, cloner *mockCloner) *runner.Runner {
	t.Helper()
	return runner.New(cloner, runner.Config{
		WorkspaceDir: t.TempDir(),
		ZigzagBin:    mockZigzag(t),
		ReportsDir:   t.TempDir(),
	})
}

func TestRun_success(t *testing.T) {
	cloner := &mockCloner{token: "ghs_test"}
	r := newRunner(t, cloner)

	event := github.WebhookEvent{
		RepoName:       "org/repo",
		CloneURL:       "https://github.com/org/repo.git",
		Branch:         "main",
		CommitSHA:      "abc123",
		InstallationID: 42,
	}

	if err := r.Run(context.Background(), event); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRun_tokenError(t *testing.T) {
	cloner := &mockCloner{tokenErr: errors.New("auth failed")}
	r := newRunner(t, cloner)

	err := r.Run(context.Background(), github.WebhookEvent{})
	if err == nil {
		t.Fatal("expected error from token failure, got nil")
	}
}

func TestRun_cloneError(t *testing.T) {
	cloner := &mockCloner{token: "ghs_test", cloneErr: errors.New("clone failed")}
	r := newRunner(t, cloner)

	err := r.Run(context.Background(), github.WebhookEvent{InstallationID: 1})
	if err == nil {
		t.Fatal("expected error from clone failure, got nil")
	}
}

func TestRun_zigzagError(t *testing.T) {
	cloner := &mockCloner{token: "ghs_test"}

	// Write a zigzag that exits non-zero.
	dir := t.TempDir()
	failBin := filepath.Join(dir, "zigzag")
	os.WriteFile(failBin, []byte("#!/bin/sh\nexit 1\n"), 0o755)

	r := runner.New(cloner, runner.Config{
		WorkspaceDir: t.TempDir(),
		ZigzagBin:    failBin,
		ReportsDir:   t.TempDir(),
	})

	err := r.Run(context.Background(), github.WebhookEvent{InstallationID: 1})
	if err == nil {
		t.Fatal("expected error from zigzag failure, got nil")
	}
}
