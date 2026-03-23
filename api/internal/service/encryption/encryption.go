package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
)

const KeyByteSize = 32

// Encrypter encrypts and decrypts data.
type Encrypter interface {
	Encrypt(plaintext []byte) ([]byte, error)
	Decrypt(ciphertext []byte) ([]byte, error)
}

var _ Encrypter = (*Service)(nil)

// Service encrypts/decrypts using AES-256-GCM.
// Ciphertext format: nonce (12 bytes) || ciphertext.
type Service struct {
	aead cipher.AEAD
}

// New creates an encryption Service. key must be exactly 32 bytes.
func New(key []byte) (*Service, error) {
	if len(key) != KeyByteSize {
		return nil, fmt.Errorf("encryption: key must be %d bytes, got %d", KeyByteSize, len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Service{aead: aead}, nil
}

// Encrypt returns nonce || ciphertext.
func (s *Service) Encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("encryption: nonce: %w", err)
	}
	return s.aead.Seal(nonce, nonce, plaintext, nil), nil
}

// Decrypt expects nonce || ciphertext as produced by Encrypt.
func (s *Service) Decrypt(data []byte) ([]byte, error) {
	ns := s.aead.NonceSize()
	if len(data) < ns {
		return nil, errors.New("encryption: ciphertext too short")
	}
	plain, err := s.aead.Open(nil, data[:ns], data[ns:], nil)
	if err != nil {
		return nil, fmt.Errorf("encryption: decrypt: %w", err)
	}
	return plain, nil
}
