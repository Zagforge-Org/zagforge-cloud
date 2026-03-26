package mfa

import (
	"strings"
	"testing"
)

func TestGenerateBackupCodes_count(t *testing.T) {
	codes, err := GenerateBackupCodes()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(codes) != backupCodeCount {
		t.Errorf("expected %d codes, got %d", backupCodeCount, len(codes))
	}
}

func TestGenerateBackupCodes_format(t *testing.T) {
	codes, err := GenerateBackupCodes()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for i, c := range codes {
		// Format: xxxx-xxxx (9 chars with dash).
		if len(c.Plain) != 9 {
			t.Errorf("code[%d]: expected length 9, got %d: %s", i, len(c.Plain), c.Plain)
		}
		if !strings.Contains(c.Plain, "-") {
			t.Errorf("code[%d]: expected dash separator: %s", i, c.Plain)
		}
		if c.Hash == "" {
			t.Errorf("code[%d]: expected non-empty hash", i)
		}
	}
}

func TestGenerateBackupCodes_uniqueness(t *testing.T) {
	codes, _ := GenerateBackupCodes()

	seen := make(map[string]bool, len(codes))
	for _, c := range codes {
		if seen[c.Plain] {
			t.Errorf("duplicate code: %s", c.Plain)
		}
		seen[c.Plain] = true
	}
}

func TestVerifyBackupCode_valid(t *testing.T) {
	codes, _ := GenerateBackupCodes()

	for _, c := range codes {
		if err := VerifyBackupCode(c.Plain, c.Hash); err != nil {
			t.Errorf("expected valid code %q, got error: %v", c.Plain, err)
		}
	}
}

func TestVerifyBackupCode_invalid(t *testing.T) {
	codes, _ := GenerateBackupCodes()
	if err := VerifyBackupCode("wrong-code", codes[0].Hash); err == nil {
		t.Error("expected error for wrong code")
	}
}

func TestVerifyBackupCode_wrongHash(t *testing.T) {
	codes, _ := GenerateBackupCodes()
	// Verify code[0] against code[1]'s hash.
	if len(codes) < 2 {
		t.Skip("need at least 2 codes")
	}
	if err := VerifyBackupCode(codes[0].Plain, codes[1].Hash); err == nil {
		t.Error("expected error for mismatched hash")
	}
}

func TestFormatCode(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"abcd1234", "abcd-1234"},
		{"12345678", "1234-5678"},
		{"short", "short"}, // not 8 chars, returned as-is
	}
	for _, tt := range tests {
		got := formatCode(tt.input)
		if got != tt.expected {
			t.Errorf("formatCode(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
