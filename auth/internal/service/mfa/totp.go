package mfa

import (
	"errors"
	"fmt"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

const (
	totpIssuer = "ZagForge"
	totpDigits = otp.DigitsSix
	totpPeriod = 30
	totpAlgo   = otp.AlgorithmSHA1
	totpSkew   = 1 // allow +/- 1 time step for clock drift
)

var (
	errGenerateKey = errors.New("failed to generate TOTP key")
	errInvalidCode = errors.New("invalid TOTP code")
)

// TOTPKey holds a newly generated TOTP secret and its provisioning URI.
type TOTPKey struct {
	Secret string // base32-encoded secret
	URI    string // otpauth:// URI for QR code generation
}

// GenerateTOTPKey creates a new TOTP key for a user.
func GenerateTOTPKey(email string) (TOTPKey, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      totpIssuer,
		AccountName: email,
		Digits:      totpDigits,
		Period:      totpPeriod,
		Algorithm:   totpAlgo,
	})
	if err != nil {
		return TOTPKey{}, fmt.Errorf("%w: %w", errGenerateKey, err)
	}

	return TOTPKey{
		Secret: key.Secret(),
		URI:    key.URL(),
	}, nil
}

// ValidateTOTPCode checks whether a TOTP code is valid for the given secret.
func ValidateTOTPCode(code, secret string) error {
	valid, err := totp.ValidateCustom(code, secret, time.Now(), totp.ValidateOpts{
		Digits:    totpDigits,
		Period:    totpPeriod,
		Algorithm: totpAlgo,
		Skew:      totpSkew,
	})
	if err != nil {
		return fmt.Errorf("%w: %w", errInvalidCode, err)
	}
	if !valid {
		return errInvalidCode
	}
	return nil
}
