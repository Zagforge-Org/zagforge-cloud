package contexttokens

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"

	handlerpkg "github.com/LegationPro/zagforge/api/internal/handler"
)

func TestGenerateToken_Format(t *testing.T) {
	tok, err := generateToken()
	if err != nil {
		t.Fatalf("generateToken: %v", err)
	}
	if !strings.HasPrefix(tok, "zf_ctx_") {
		t.Errorf("token %q missing prefix zf_ctx_", tok)
	}
	// After prefix, the rest should be valid base64url (24 random bytes = 32 base64 chars).
	encoded := strings.TrimPrefix(tok, "zf_ctx_")
	decoded, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("invalid base64url: %v", err)
	}
	if len(decoded) != 24 {
		t.Errorf("expected 24 random bytes, got %d", len(decoded))
	}
}

func TestGenerateToken_Unique(t *testing.T) {
	t1, _ := generateToken()
	t2, _ := generateToken()
	if t1 == t2 {
		t.Error("expected unique tokens")
	}
}

func TestSha256Hash_Deterministic(t *testing.T) {
	h1 := handlerpkg.SHA256Hash("zf_ctx_abc")
	h2 := handlerpkg.SHA256Hash("zf_ctx_abc")
	if h1 != h2 {
		t.Error("same input produced different hashes")
	}
}

func TestSha256Hash_MatchesStdlib(t *testing.T) {
	input := "zf_ctx_testtoken"
	expected := sha256.Sum256([]byte(input))
	want := hex.EncodeToString(expected[:])
	got := handlerpkg.SHA256Hash(input)
	if got != want {
		t.Errorf("SHA256Hash(%q) = %q, want %q", input, got, want)
	}
}

func TestSha256Hash_DifferentInputs(t *testing.T) {
	h1 := handlerpkg.SHA256Hash("token_a")
	h2 := handlerpkg.SHA256Hash("token_b")
	if h1 == h2 {
		t.Error("different inputs produced the same hash")
	}
}

func TestGenerateToken_HashRoundtrip(t *testing.T) {
	tok, err := generateToken()
	if err != nil {
		t.Fatalf("generateToken: %v", err)
	}
	hash := handlerpkg.SHA256Hash(tok)
	if hash == "" {
		t.Error("hash should not be empty")
	}
	if len(hash) != 64 {
		t.Errorf("expected 64 hex chars, got %d", len(hash))
	}
}
