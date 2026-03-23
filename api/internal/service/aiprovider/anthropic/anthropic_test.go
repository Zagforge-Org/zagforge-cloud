package anthropic_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/LegationPro/zagforge/api/internal/service/aiprovider/anthropic"
)

func TestStream_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "sk-ant-test" {
			t.Errorf("x-api-key = %q", r.Header.Get("x-api-key"))
		}
		if r.Header.Get("anthropic-version") != "2023-06-01" {
			t.Errorf("anthropic-version = %q", r.Header.Get("anthropic-version"))
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "event: content_block_delta\n")
		fmt.Fprint(w, "data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"Hello\"}}\n\n")
		fmt.Fprint(w, "event: content_block_delta\n")
		fmt.Fprint(w, "data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\" Claude\"}}\n\n")
		fmt.Fprint(w, "event: message_stop\n")
		fmt.Fprint(w, "data: {}\n\n")
	}))
	defer srv.Close()

	p := &anthropic.Provider{Model: "claude-sonnet-4-20250514", Endpoint: srv.URL}
	w := httptest.NewRecorder()

	if err := p.Stream(context.Background(), w, "sk-ant-test", "hi"); err != nil {
		t.Fatalf("Stream: %v", err)
	}

	body := w.Body.String()
	if !strings.Contains(body, "data: Hello\n\n") {
		t.Errorf("missing first chunk:\n%s", body)
	}
	if !strings.Contains(body, "data:  Claude\n\n") {
		t.Errorf("missing second chunk:\n%s", body)
	}
	if !strings.HasSuffix(body, "data: [DONE]\n\n") {
		t.Errorf("missing DONE:\n%s", body)
	}
}

func TestStream_SkipsNonDeltaEvents(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "event: message_start\n")
		fmt.Fprint(w, "data: {\"type\":\"message_start\"}\n\n")
		fmt.Fprint(w, "event: content_block_start\n")
		fmt.Fprint(w, "data: {\"type\":\"content_block_start\"}\n\n")
		fmt.Fprint(w, "event: content_block_delta\n")
		fmt.Fprint(w, "data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"only this\"}}\n\n")
		fmt.Fprint(w, "event: message_stop\n")
		fmt.Fprint(w, "data: {}\n\n")
	}))
	defer srv.Close()

	p := &anthropic.Provider{Model: "test", Endpoint: srv.URL}
	w := httptest.NewRecorder()

	p.Stream(context.Background(), w, "key", "hi")

	body := w.Body.String()
	if !strings.Contains(body, "data: only this\n\n") {
		t.Errorf("expected only delta content:\n%s", body)
	}
	// Should not contain message_start or content_block_start data
	if strings.Contains(body, "message_start") {
		t.Errorf("should not forward non-delta events:\n%s", body)
	}
}

func TestStream_ErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	p := &anthropic.Provider{Model: "test", Endpoint: srv.URL}
	w := httptest.NewRecorder()

	err := p.Stream(context.Background(), w, "key", "hi")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(w.Body.String(), "[ERROR]") {
		t.Errorf("expected SSE error:\n%s", w.Body.String())
	}
}

func TestStream_EmptyKey(t *testing.T) {
	p := anthropic.New()
	w := httptest.NewRecorder()
	if err := p.Stream(context.Background(), w, "", "hi"); err == nil {
		t.Fatal("expected error for empty key")
	}
}
