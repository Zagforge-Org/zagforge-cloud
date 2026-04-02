package token

import (
	"testing"
)

func TestHashToken_deterministic(t *testing.T) {
	h1 := HashToken("my-token")
	h2 := HashToken("my-token")
	if h1 != h2 {
		t.Error("expected same hash for same input")
	}
}

func TestHashToken_differentInputs(t *testing.T) {
	h1 := HashToken("token-a")
	h2 := HashToken("token-b")
	if h1 == h2 {
		t.Error("expected different hashes for different inputs")
	}
}

func TestHashToken_length(t *testing.T) {
	h := HashToken("test")
	// SHA-256 hex = 64 characters.
	if len(h) != 64 {
		t.Errorf("expected hash length 64, got %d", len(h))
	}
}

func TestHashToken_empty(t *testing.T) {
	h := HashToken("")
	if h == "" {
		t.Error("expected non-empty hash for empty input")
	}
	if len(h) != 64 {
		t.Errorf("expected hash length 64, got %d", len(h))
	}
}
