package xai_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/LegationPro/zagforge/api/internal/service/aiprovider/xai"
)

func TestStream_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer xai-key" {
			t.Errorf("auth = %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"Grok\"}}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\" says hi\"}}]}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	p := &xai.Provider{Model: "grok-3", Endpoint: srv.URL}
	w := httptest.NewRecorder()

	if err := p.Stream(context.Background(), w, "xai-key", "hi"); err != nil {
		t.Fatalf("Stream: %v", err)
	}

	body := w.Body.String()
	if !strings.Contains(body, "data: Grok\n\n") {
		t.Errorf("missing first chunk:\n%s", body)
	}
	if !strings.Contains(body, "data:  says hi\n\n") {
		t.Errorf("missing second chunk:\n%s", body)
	}
	if !strings.HasSuffix(body, "data: [DONE]\n\n") {
		t.Errorf("missing DONE:\n%s", body)
	}
}

func TestStream_ErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	p := &xai.Provider{Model: "grok-3", Endpoint: srv.URL}
	w := httptest.NewRecorder()

	err := p.Stream(context.Background(), w, "bad-key", "hi")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(w.Body.String(), "[ERROR]") {
		t.Errorf("expected SSE error:\n%s", w.Body.String())
	}
}

func TestStream_EmptyKey(t *testing.T) {
	p := xai.New()
	w := httptest.NewRecorder()
	if err := p.Stream(context.Background(), w, "", "hi"); err == nil {
		t.Fatal("expected error for empty key")
	}
}
