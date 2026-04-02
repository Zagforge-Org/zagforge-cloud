package token

import (
	"testing"
)

func TestGenerateRefreshToken_uniqueness(t *testing.T) {
	svc := newTestService(t)

	tok1, err := svc.GenerateRefreshToken()
	if err != nil {
		t.Fatalf("generate 1: %v", err)
	}

	tok2, err := svc.GenerateRefreshToken()
	if err != nil {
		t.Fatalf("generate 2: %v", err)
	}

	if tok1.Raw == tok2.Raw {
		t.Error("expected unique raw tokens")
	}
	if tok1.Hash == tok2.Hash {
		t.Error("expected unique hashes")
	}
}

func TestGenerateRefreshToken_hashDeterministic(t *testing.T) {
	svc := newTestService(t)

	tok, err := svc.GenerateRefreshToken()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	// Hashing the same raw value should produce the same hash.
	if HashToken(tok.Raw) != tok.Hash {
		t.Error("expected deterministic hash")
	}
}

func TestGenerateRefreshToken_nonEmpty(t *testing.T) {
	svc := newTestService(t)

	tok, err := svc.GenerateRefreshToken()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	if tok.Raw == "" {
		t.Error("expected non-empty raw token")
	}
	if tok.Hash == "" {
		t.Error("expected non-empty hash")
	}
}
