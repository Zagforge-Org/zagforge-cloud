package mfa

import (
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
)

func TestGenerateTOTPKey_success(t *testing.T) {
	key, err := GenerateTOTPKey("user@example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key.Secret == "" {
		t.Error("expected non-empty secret")
	}
	if key.URI == "" {
		t.Error("expected non-empty URI")
	}
}

func TestGenerateTOTPKey_containsIssuer(t *testing.T) {
	key, err := GenerateTOTPKey("user@example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(key.URI) < 10 {
		t.Fatalf("URI too short: %s", key.URI)
	}

	// URI should contain the issuer.
	if !containsSubstring(key.URI, totpIssuer) {
		t.Errorf("URI should contain issuer %q: %s", totpIssuer, key.URI)
	}
}

func TestGenerateTOTPKey_uniqueSecrets(t *testing.T) {
	k1, _ := GenerateTOTPKey("a@b.com")
	k2, _ := GenerateTOTPKey("a@b.com")
	if k1.Secret == k2.Secret {
		t.Error("expected unique secrets")
	}
}

func TestValidateTOTPCode_validCode(t *testing.T) {
	key, err := GenerateTOTPKey("user@example.com")
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	// Generate a valid code for now.
	code, err := totp.GenerateCode(key.Secret, time.Now())
	if err != nil {
		t.Fatalf("generate code: %v", err)
	}

	if err := ValidateTOTPCode(code, key.Secret); err != nil {
		t.Errorf("expected valid code, got error: %v", err)
	}
}

func TestValidateTOTPCode_invalidCode(t *testing.T) {
	key, _ := GenerateTOTPKey("user@example.com")

	err := ValidateTOTPCode("000000", key.Secret)
	if err == nil {
		t.Error("expected error for invalid code")
	}
}

func TestValidateTOTPCode_wrongSecret(t *testing.T) {
	k1, _ := GenerateTOTPKey("a@b.com")
	k2, _ := GenerateTOTPKey("c@d.com")

	code, _ := totp.GenerateCode(k1.Secret, time.Now())

	err := ValidateTOTPCode(code, k2.Secret)
	if err == nil {
		t.Error("expected error for code generated with different secret")
	}
}

func containsSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
