package assembly_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/LegationPro/zagforge/api/internal/service/assembly"
)

type stubFetcher struct {
	blobs map[string]string
}

func (f *stubFetcher) FetchBlob(_ context.Context, sha string) (string, error) {
	content, ok := f.blobs[sha]
	if !ok {
		return "", errors.New("blob not found: " + sha)
	}
	return content, nil
}

func TestAssemble_SingleFile(t *testing.T) {
	fetcher := &stubFetcher{blobs: map[string]string{
		"abc123": "package main\n\nfunc main() {}",
	}}
	files := []assembly.FileEntry{
		{Path: "main.go", Language: "go", Lines: 3, SHA: "abc123"},
	}

	var buf bytes.Buffer
	err := assembly.Assemble(context.Background(), "org/repo", "deadbeef12345678", files, fetcher, &buf)
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "# Codebase Snapshot — org/repo @ deadbeef") {
		t.Errorf("missing header, got:\n%s", out)
	}
	if !strings.Contains(out, "## main.go") {
		t.Errorf("missing file heading, got:\n%s", out)
	}
	if !strings.Contains(out, "```go\npackage main") {
		t.Errorf("missing fenced code block, got:\n%s", out)
	}
}

func TestAssemble_MultipleFiles(t *testing.T) {
	fetcher := &stubFetcher{blobs: map[string]string{
		"sha1": "content one",
		"sha2": "content two",
	}}
	files := []assembly.FileEntry{
		{Path: "a.go", Language: "go", SHA: "sha1"},
		{Path: "b.py", Language: "python", SHA: "sha2"},
	}

	var buf bytes.Buffer
	err := assembly.Assemble(context.Background(), "org/repo", "abcdef1234567890", files, fetcher, &buf)
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "## a.go") || !strings.Contains(out, "## b.py") {
		t.Errorf("missing file headings, got:\n%s", out)
	}
	if !strings.Contains(out, "```go\ncontent one") {
		t.Errorf("missing first file content, got:\n%s", out)
	}
	if !strings.Contains(out, "```python\ncontent two") {
		t.Errorf("missing second file content, got:\n%s", out)
	}
}

func TestAssemble_EmptyLanguageFallsBackToText(t *testing.T) {
	fetcher := &stubFetcher{blobs: map[string]string{"sha1": "data"}}
	files := []assembly.FileEntry{
		{Path: "Makefile", Language: "", SHA: "sha1"},
	}

	var buf bytes.Buffer
	assembly.Assemble(context.Background(), "org/repo", "abcdef1234567890", files, fetcher, &buf)

	if !strings.Contains(buf.String(), "```text\ndata") {
		t.Errorf("expected language fallback to 'text', got:\n%s", buf.String())
	}
}

func TestAssemble_ShortCommitSha(t *testing.T) {
	fetcher := &stubFetcher{blobs: map[string]string{}}
	var buf bytes.Buffer
	assembly.Assemble(context.Background(), "org/repo", "abc", nil, fetcher, &buf)

	if !strings.Contains(buf.String(), "@ abc") {
		t.Errorf("short SHA should be used as-is, got:\n%s", buf.String())
	}
}

func TestAssemble_NoFiles(t *testing.T) {
	fetcher := &stubFetcher{blobs: map[string]string{}}
	var buf bytes.Buffer
	err := assembly.Assemble(context.Background(), "org/repo", "deadbeef12345678", nil, fetcher, &buf)
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "# Codebase Snapshot") {
		t.Errorf("expected header even with no files, got:\n%s", out)
	}
}

func TestAssemble_FetchError(t *testing.T) {
	fetcher := &stubFetcher{blobs: map[string]string{}}
	files := []assembly.FileEntry{
		{Path: "missing.go", SHA: "nonexistent"},
	}

	var buf bytes.Buffer
	err := assembly.Assemble(context.Background(), "org/repo", "deadbeef12345678", files, fetcher, &buf)
	if err == nil {
		t.Fatal("expected error when blob fetch fails")
	}
	if !strings.Contains(err.Error(), "missing.go") {
		t.Errorf("error should mention file path, got: %v", err)
	}
}
