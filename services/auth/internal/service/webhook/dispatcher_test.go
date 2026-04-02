package webhook

import (
	"strings"
	"testing"
)

func TestSign_format(t *testing.T) {
	sig := Sign([]byte(`{"event":"test"}`), "secret123")

	if !strings.HasPrefix(sig, signaturePrefix) {
		t.Errorf("expected %s prefix, got %q", signaturePrefix, sig)
	}

	// "sha256=" (7) + 64 hex chars = 71
	if len(sig) != 71 {
		t.Errorf("expected length 71, got %d", len(sig))
	}
}

func TestSign_deterministic(t *testing.T) {
	payload := []byte(`{"test":true}`)
	secret := "my-secret"

	s1 := Sign(payload, secret)
	s2 := Sign(payload, secret)

	if s1 != s2 {
		t.Error("expected same signature for same input")
	}
}

func TestSign_differentPayload(t *testing.T) {
	secret := "my-secret"

	s1 := Sign([]byte(`{"a":1}`), secret)
	s2 := Sign([]byte(`{"b":2}`), secret)

	if s1 == s2 {
		t.Error("expected different signatures for different payloads")
	}
}

func TestSign_differentSecret(t *testing.T) {
	payload := []byte(`{"test":true}`)

	s1 := Sign(payload, "secret-a")
	s2 := Sign(payload, "secret-b")

	if s1 == s2 {
		t.Error("expected different signatures for different secrets")
	}
}

func TestIsSuccessStatus(t *testing.T) {
	tests := []struct {
		status   int
		expected bool
	}{
		{200, true},
		{201, true},
		{204, true},
		{299, true},
		{300, false},
		{301, false},
		{400, false},
		{404, false},
		{500, false},
		{199, false},
		{0, false},
	}

	for _, tt := range tests {
		got := isSuccessStatus(tt.status)
		if got != tt.expected {
			t.Errorf("isSuccessStatus(%d) = %v, want %v", tt.status, got, tt.expected)
		}
	}
}
