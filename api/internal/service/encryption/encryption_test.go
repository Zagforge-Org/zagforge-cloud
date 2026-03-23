package encryption_test

import (
	"testing"

	"github.com/LegationPro/zagforge/api/internal/service/encryption"
)

func TestEncryptDecryptRoundtrip(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	svc, err := encryption.New(key)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	plaintext := []byte("sk-ant-api01-supersecret")
	cipher, err := svc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	got, err := svc.Decrypt(cipher)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if string(got) != string(plaintext) {
		t.Errorf("got %q, want %q", got, plaintext)
	}
}

func TestEncryptProducesDifferentCiphertexts(t *testing.T) {
	key := make([]byte, 32)
	svc, _ := encryption.New(key)

	plaintext := []byte("same-input")
	c1, _ := svc.Encrypt(plaintext)
	c2, _ := svc.Encrypt(plaintext)

	if string(c1) == string(c2) {
		t.Fatal("expected different ciphertexts for same plaintext (random nonce)")
	}
}

func TestDecryptTamperedCiphertext(t *testing.T) {
	key := make([]byte, 32)
	svc, _ := encryption.New(key)
	cipher, _ := svc.Encrypt([]byte("secret"))
	cipher[len(cipher)-1] ^= 0xFF
	if _, err := svc.Decrypt(cipher); err == nil {
		t.Fatal("expected error on tampered ciphertext")
	}
}

func TestDecryptTooShort(t *testing.T) {
	key := make([]byte, 32)
	svc, _ := encryption.New(key)
	if _, err := svc.Decrypt([]byte("short")); err == nil {
		t.Fatal("expected error on short ciphertext")
	}
}

func TestNewRejectsShortKey(t *testing.T) {
	if _, err := encryption.New([]byte("tooshort")); err == nil {
		t.Fatal("expected error for short key")
	}
}

func TestNewRejectsLongKey(t *testing.T) {
	if _, err := encryption.New(make([]byte, 64)); err == nil {
		t.Fatal("expected error for long key")
	}
}
