package openai_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/LegationPro/zagforge/api/internal/service/aiprovider/openai"
)

func TestStream_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("auth = %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("content-type = %q", r.Header.Get("Content-Type"))
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"Hello\"}}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\" world\"}}]}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	p := &openai.Provider{Model: "gpt-4o", Endpoint: srv.URL}
	w := httptest.NewRecorder()

	if err := p.Stream(context.Background(), w, "test-key", "hi"); err != nil {
		t.Fatalf("Stream: %v", err)
	}

	body := w.Body.String()
	if !strings.Contains(body, "data: Hello\n\n") {
		t.Errorf("missing first chunk:\n%s", body)
	}
	if !strings.Contains(body, "data:  world\n\n") {
		t.Errorf("missing second chunk:\n%s", body)
	}
	if !strings.HasSuffix(body, "data: [DONE]\n\n") {
		t.Errorf("missing DONE:\n%s", body)
	}
}

func TestStream_ErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	p := &openai.Provider{Model: "gpt-4o", Endpoint: srv.URL}
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
	p := openai.New()
	w := httptest.NewRecorder()
	if err := p.Stream(context.Background(), w, "", "hi"); err == nil {
		t.Fatal("expected error for empty key")
	}
}

func TestStream_EmptyPrompt(t *testing.T) {
	p := openai.New()
	w := httptest.NewRecorder()
	if err := p.Stream(context.Background(), w, "key", ""); err == nil {
		t.Fatal("expected error for empty prompt")
	}
}
