package testutil

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"testing"
)

// KeyPair holds base64-encoded PEM Ed25519 keys for testing.
type KeyPair struct {
	PrivateKeyBase64 string
	PublicKeyBase64  string
	PublicKey        ed25519.PublicKey
	PrivateKey       ed25519.PrivateKey
}

// GenerateKeyPair creates a fresh Ed25519 key pair for tests.
func GenerateKeyPair(t *testing.T) KeyPair {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate ed25519 key: %v", err)
	}

	privDER, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatalf("marshal private key: %v", err)
	}
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privDER})

	pubDER, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER})

	return KeyPair{
		PrivateKeyBase64: base64.StdEncoding.EncodeToString(privPEM),
		PublicKeyBase64:  base64.StdEncoding.EncodeToString(pubPEM),
		PublicKey:        pub,
		PrivateKey:       priv,
	}
}
