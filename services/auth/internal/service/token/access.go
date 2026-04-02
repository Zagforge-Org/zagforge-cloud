package token

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/LegationPro/zagforge/shared/go/authclaims"
)

// AccessTokenParams holds the data needed to issue an access token.
type AccessTokenParams struct {
	UserID string
	Email  string
	Name   string
	Org    authclaims.OrgClaim
	Teams  []string
	MFA    bool
}

// IssueAccessToken creates a signed JWT access token.
func (s *Service) IssueAccessToken(p AccessTokenParams) (string, error) {
	now := time.Now()
	claims := authclaims.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.issuer,
			Subject:   p.UserID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.accessTTL)),
			ID:        uuid.New().String(),
		},
		Email: p.Email,
		Name:  p.Name,
		Org:   p.Org,
		Teams: p.Teams,
		MFA:   p.MFA,
	}

	t := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	return t.SignedString(s.privateKey)
}
