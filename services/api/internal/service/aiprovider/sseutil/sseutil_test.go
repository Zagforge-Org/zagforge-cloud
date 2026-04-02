package sseutil_test

import (
	"net/http/httptest"
	"testing"

	"github.com/LegationPro/zagforge/api/internal/service/aiprovider/sseutil"
)

func TestValidateInput(t *testing.T) {
	tests := []struct {
		name    string
		apiKey  string
		prompt  string
		wantErr bool
	}{
		{"valid", "sk-key", "hello", false},
		{"empty key", "", "hello", true},
		{"empty prompt", "sk-key", "", true},
		{"both empty", "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sseutil.ValidateInput(tt.apiKey, tt.prompt)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateInput(%q, %q) err=%v, wantErr=%v", tt.apiKey, tt.prompt, err, tt.wantErr)
			}
		})
	}
}

func TestParseSSELine(t *testing.T) {
	tests := []struct {
		line string
		data string
		ok   bool
	}{
		{"data: hello", "hello", true},
		{"data: [DONE]", "[DONE]", true},
		{"data: ", "", true},
		{"event: content_block_delta", "event: content_block_delta", false},
		{"", "", false},
		{": comment", ": comment", false},
	}
	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			data, ok := sseutil.ParseSSELine(tt.line)
			if ok != tt.ok || data != tt.data {
				t.Errorf("ParseSSELine(%q) = (%q, %v), want (%q, %v)", tt.line, data, ok, tt.data, tt.ok)
			}
		})
	}
}

func TestWriteChunk(t *testing.T) {
	w := httptest.NewRecorder()
	sseutil.WriteChunk(w, "hello world")
	if got, want := w.Body.String(), "data: hello world\n\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestWriteDone(t *testing.T) {
	w := httptest.NewRecorder()
	sseutil.WriteDone(w)
	if got, want := w.Body.String(), "data: [DONE]\n\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()
	sseutil.WriteError(w, "something broke")
	if got, want := w.Body.String(), "data: [ERROR] something broke\n\ndata: [DONE]\n\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
