package token

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
)

var errGenerateRefreshToken = errors.New("generate refresh token")

// RefreshToken holds the raw token value and its SHA-256 hash.
type RefreshToken struct {
	Raw  string // The opaque token sent to the client.
	Hash string // SHA-256 hex digest stored in the database.
}

// GenerateRefreshToken creates a cryptographically random refresh token.
func (s *Service) GenerateRefreshToken() (RefreshToken, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return RefreshToken{}, fmt.Errorf("%w: %w", errGenerateRefreshToken, err)
	}
	raw := base64.RawURLEncoding.EncodeToString(b)
	return RefreshToken{
		Raw:  raw,
		Hash: HashToken(raw),
	}, nil
}
