package gemini_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/LegationPro/zagforge/api/internal/service/aiprovider/gemini"
)

func TestStream_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.RawQuery, "key=gem-key") {
			t.Errorf("expected API key in query, got %q", r.URL.RawQuery)
		}
		if r.Header.Get("Authorization") != "" {
			t.Error("gemini should not send Authorization header")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"Hello\"}]}}]}\n\n")
		fmt.Fprint(w, "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\" Gemini\"}]}}]}\n\n")
	}))
	defer srv.Close()

	p := &gemini.Provider{Model: "gemini-2.0-flash", BaseURL: srv.URL}
	w := httptest.NewRecorder()

	if err := p.Stream(context.Background(), w, "gem-key", "hi"); err != nil {
		t.Fatalf("Stream: %v", err)
	}

	body := w.Body.String()
	if !strings.Contains(body, "data: Hello\n\n") {
		t.Errorf("missing first chunk:\n%s", body)
	}
	if !strings.Contains(body, "data:  Gemini\n\n") {
		t.Errorf("missing second chunk:\n%s", body)
	}
	if !strings.HasSuffix(body, "data: [DONE]\n\n") {
		t.Errorf("missing DONE:\n%s", body)
	}
}

func TestStream_ErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	p := &gemini.Provider{Model: "test", BaseURL: srv.URL}
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
	p := gemini.New()
	w := httptest.NewRecorder()
	if err := p.Stream(context.Background(), w, "", "hi"); err == nil {
		t.Fatal("expected error for empty key")
	}
}

func TestStream_EmptyPrompt(t *testing.T) {
	p := gemini.New()
	w := httptest.NewRecorder()
	if err := p.Stream(context.Background(), w, "key", ""); err == nil {
		t.Fatal("expected error for empty prompt")
	}
}
