package token

import (
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	// AudienceMFAChallenge is the JWT audience value for MFA challenge tokens.
	AudienceMFAChallenge = "mfa_challenge"

	mfaChallengeTTL = 5 * time.Minute
)

var (
	errInvalidMFAToken         = errors.New("invalid mfa challenge token")
	errInvalidClaimsType       = errors.New("invalid claims type")
	errNotMFAToken             = errors.New("token is not an mfa challenge token")
	errUnexpectedSigningMethod = errors.New("unexpected signing method")
)

// IssueMFAChallengeToken creates a short-lived token for the MFA challenge step.
func (s *Service) IssueMFAChallengeToken(userID string) (string, error) {
	now := time.Now()
	claims := jwt.RegisteredClaims{
		Issuer:    s.issuer,
		Subject:   userID,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(mfaChallengeTTL)),
		ID:        uuid.New().String(),
		Audience:  jwt.ClaimStrings{AudienceMFAChallenge},
	}

	t := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	return t.SignedString(s.privateKey)
}

// ValidateMFAChallengeToken verifies an MFA challenge token and returns the subject.
func (s *Service) ValidateMFAChallengeToken(tokenStr string) (string, error) {
	t, err := jwt.ParseWithClaims(tokenStr, &jwt.RegisteredClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodEd25519); !ok {
			return nil, fmt.Errorf("%w: %v", errUnexpectedSigningMethod, t.Header["alg"])
		}
		return s.publicKey, nil
	}, jwt.WithIssuer(s.issuer))
	if err != nil {
		return "", fmt.Errorf("%w: %w", errInvalidMFAToken, err)
	}

	claims, ok := t.Claims.(*jwt.RegisteredClaims)
	if !ok {
		return "", errInvalidClaimsType
	}

	aud, _ := claims.GetAudience()
	if !slices.Contains(aud, AudienceMFAChallenge) {
		return "", errNotMFAToken
	}

	return claims.Subject, nil
}
