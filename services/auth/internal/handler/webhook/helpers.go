package webhook

import (
	"crypto/rand"
	"encoding/hex"
)

func generateSecret() (string, error) {
	b := make([]byte, secretBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return secretPrefix + hex.EncodeToString(b), nil
}
