package mfa

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

const (
	backupCodeCount = 10
	backupCodeBytes = 4 // 4 bytes = 8 hex chars
	bcryptCost      = bcrypt.DefaultCost
)

var errInvalidBackupCode = errors.New("invalid backup code")

// BackupCode holds a plaintext code and its bcrypt hash.
type BackupCode struct {
	Plain string
	Hash  string
}

// GenerateBackupCodes creates a set of single-use recovery codes.
// Returns the plaintext codes (shown to user once) and their hashes (stored in DB).
func GenerateBackupCodes() ([]BackupCode, error) {
	codes := make([]BackupCode, backupCodeCount)
	for i := range codes {
		b := make([]byte, backupCodeBytes)
		if _, err := rand.Read(b); err != nil {
			return nil, fmt.Errorf("generate backup code: %w", err)
		}

		plain := formatCode(hex.EncodeToString(b))
		hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcryptCost)
		if err != nil {
			return nil, fmt.Errorf("hash backup code: %w", err)
		}

		codes[i] = BackupCode{
			Plain: plain,
			Hash:  string(hash),
		}
	}
	return codes, nil
}

// VerifyBackupCode checks a plaintext code against a bcrypt hash.
func VerifyBackupCode(plain, hash string) error {
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)); err != nil {
		return errInvalidBackupCode
	}
	return nil
}

// formatCode formats 8 hex chars as "xxxx-xxxx" for readability.
func formatCode(s string) string {
	if len(s) != 8 {
		return s
	}
	return s[:4] + "-" + s[4:]
}
