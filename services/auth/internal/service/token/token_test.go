package token

import (
	"testing"
	"time"

	"github.com/LegationPro/zagforge/auth/internal/testutil"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	kp := testutil.GenerateKeyPair(t)
	svc, err := New(kp.PrivateKeyBase64, kp.PublicKeyBase64, "test-issuer", 15*time.Minute, 7*24*time.Hour)
	if err != nil {
		t.Fatalf("create token service: %v", err)
	}
	return svc
}

func TestNew_validKeys(t *testing.T) {
	svc := newTestService(t)
	if svc.Issuer() != "test-issuer" {
		t.Errorf("expected issuer %q, got %q", "test-issuer", svc.Issuer())
	}
	if svc.RefreshTokenTTL() != 7*24*time.Hour {
		t.Errorf("expected refresh TTL 168h, got %v", svc.RefreshTokenTTL())
	}
	if svc.PublicKey() == nil {
		t.Error("expected non-nil public key")
	}
}

func TestNew_invalidPrivateKey(t *testing.T) {
	kp := testutil.GenerateKeyPair(t)
	_, err := New("not-base64!", kp.PublicKeyBase64, "test", time.Minute, time.Hour)
	if err == nil {
		t.Fatal("expected error for invalid private key base64")
	}
}

func TestNew_invalidPublicKey(t *testing.T) {
	kp := testutil.GenerateKeyPair(t)
	_, err := New(kp.PrivateKeyBase64, "not-base64!", "test", time.Minute, time.Hour)
	if err == nil {
		t.Fatal("expected error for invalid public key base64")
	}
}

func TestNew_wrongKeyFormat(t *testing.T) {
	// Valid base64 but not a PEM key.
	_, err := New("dGVzdA==", "dGVzdA==", "test", time.Minute, time.Hour)
	if err == nil {
		t.Fatal("expected error for non-PEM key")
	}
}
