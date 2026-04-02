package sseutil

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

var (
	ErrEmptyAPIKey = errors.New("api key must not be empty")
	ErrEmptyPrompt = errors.New("prompt must not be empty")
)

// ValidateInput checks that apiKey and prompt are non-empty.
func ValidateInput(apiKey, prompt string) error {
	if apiKey == "" {
		return ErrEmptyAPIKey
	}
	if prompt == "" {
		return ErrEmptyPrompt
	}
	return nil
}

// NewJSONRequest builds an HTTP POST request with JSON body and the given headers.
func NewJSONRequest(ctx context.Context, url string, body any, headers map[string]string) (*http.Request, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request body: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return req, nil
}

// ParseSSELine extracts the data payload from an SSE line.
// Returns the data and true if the line is a "data: " line, or ("", false) otherwise.
func ParseSSELine(line string) (string, bool) {
	return strings.CutPrefix(line, "data: ")
}

// WriteChunk writes a single SSE data chunk and flushes.
func WriteChunk(w http.ResponseWriter, text string) {
	_, _ = io.WriteString(w, "data: "+text+"\n\n")
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

// WriteDone writes the SSE done sentinel and flushes.
func WriteDone(w http.ResponseWriter) {
	_, _ = io.WriteString(w, "data: [DONE]\n\n")
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

// WriteError writes an error chunk followed by done.
func WriteError(w http.ResponseWriter, msg string) {
	WriteChunk(w, "[ERROR] "+msg)
	WriteDone(w)
}
