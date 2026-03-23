package assembly

import (
	"context"
	"fmt"
	"io"
)

// FileEntry represents one entry from the snashot file tree.
type FileEntry struct {
	Path     string `json:"path"`
	Language string `json:"language"`
	Lines    int    `json:"lines"`
	SHA      string `json:"sha"`
}

// Fetcher retrieves raw file content by its Git blob SHA.
type Fetcher interface {
	FetchBlob(ctx context.Context, sha string) (string, error)
}

// FetcherFunc adapts a plain function to Fetcher.
type FetcherFunc func(ctx context.Context, sha string) (string, error)

func (f FetcherFunc) FetchBlob(ctx context.Context, sha string) (string, error) {
	return f(ctx, sha)
}

// Assemble writes a report.llm.md-style markdown document to w.
// Flushes the header immediately; file content is streamed as each blob arrives.
func Assemble(ctx context.Context, repoFullName, commitSha string, files []FileEntry, fetcher Fetcher, w io.Writer) error {
	shortSha := commitSha
	if len(shortSha) > 8 {
		shortSha = shortSha[:8]
	}

	if _, err := fmt.Fprintf(w, "# Codebase Snapshot — %s @ %s\n\n", repoFullName, shortSha); err != nil {
		return err
	}
	flush(w)

	for _, f := range files {
		content, err := fetcher.FetchBlob(ctx, f.SHA)
		if err != nil {
			return fmt.Errorf("fetch %s: %w", f.Path, err)
		}
		lang := f.Language
		if lang == "" {
			lang = "text"
		}
		if _, err := fmt.Fprintf(w, "## %s\n\n```%s\n%s\n```\n\n", f.Path, lang, content); err != nil {
			return err
		}
		flush(w)
	}
	return nil
}

func flush(w io.Writer) {
	if fl, ok := w.(interface{ Flush() }); ok {
		fl.Flush()
	}
}
