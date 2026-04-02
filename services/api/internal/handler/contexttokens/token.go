package contexttokens

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

func generateToken() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return "zf_ctx_" + base64.RawURLEncoding.EncodeToString(b), nil
}
