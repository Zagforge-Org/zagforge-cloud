package encryption

import (
	"bytes"
	"testing"
)

func testKey() []byte {
	key := make([]byte, KeyByteSize)
	for i := range key {
		key[i] = byte(i)
	}
	return key
}

func TestNew_validKey(t *testing.T) {
	_, err := New(testKey())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNew_shortKey(t *testing.T) {
	_, err := New([]byte("short"))
	if err == nil {
		t.Fatal("expected error for short key")
	}
}

func TestNew_longKey(t *testing.T) {
	_, err := New(make([]byte, 64))
	if err == nil {
		t.Fatal("expected error for long key")
	}
}

func TestEncryptDecrypt_roundtrip(t *testing.T) {
	svc, _ := New(testKey())
	plaintext := []byte("super-secret-value-12345")

	cipher, err := svc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	got, err := svc.Decrypt(cipher)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}

	if !bytes.Equal(got, plaintext) {
		t.Errorf("expected %q, got %q", plaintext, got)
	}
}

func TestEncrypt_uniqueNonce(t *testing.T) {
	svc, _ := New(testKey())
	plaintext := []byte("same-input")

	c1, _ := svc.Encrypt(plaintext)
	c2, _ := svc.Encrypt(plaintext)

	if bytes.Equal(c1, c2) {
		t.Error("expected different ciphertexts due to unique nonces")
	}
}

func TestDecrypt_tamperedCiphertext(t *testing.T) {
	svc, _ := New(testKey())

	cipher, _ := svc.Encrypt([]byte("test"))
	cipher[len(cipher)-1] ^= 0xff // flip last byte

	_, err := svc.Decrypt(cipher)
	if err == nil {
		t.Fatal("expected error for tampered ciphertext")
	}
}

func TestDecrypt_tooShort(t *testing.T) {
	svc, _ := New(testKey())
	_, err := svc.Decrypt([]byte("short"))
	if err == nil {
		t.Fatal("expected error for short ciphertext")
	}
}

func TestDecrypt_wrongKey(t *testing.T) {
	svc1, _ := New(testKey())

	key2 := make([]byte, KeyByteSize)
	for i := range key2 {
		key2[i] = byte(i + 100)
	}
	svc2, _ := New(key2)

	cipher, _ := svc1.Encrypt([]byte("secret"))
	_, err := svc2.Decrypt(cipher)
	if err == nil {
		t.Fatal("expected error decrypting with wrong key")
	}
}

func TestEncryptDecrypt_emptyPlaintext(t *testing.T) {
	svc, _ := New(testKey())

	cipher, err := svc.Encrypt([]byte{})
	if err != nil {
		t.Fatalf("encrypt empty: %v", err)
	}

	got, err := svc.Decrypt(cipher)
	if err != nil {
		t.Fatalf("decrypt empty: %v", err)
	}

	if len(got) != 0 {
		t.Errorf("expected empty plaintext, got %d bytes", len(got))
	}
}
