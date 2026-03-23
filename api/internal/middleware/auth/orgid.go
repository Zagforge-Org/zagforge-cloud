package auth

import (
	"errors"

	"github.com/clerk/clerk-sdk-go/v2"
)

var ErrNoActiveOrg = errors.New("no active organization in session claims")

// ResolveClerkOrgID extracts the active Clerk organization ID from session claims.
// Returns ErrNoActiveOrg if the user has no org context in the current session.
func ResolveClerkOrgID(claims *clerk.SessionClaims) (string, error) {
	if claims.ActiveOrganizationID == "" {
		return "", ErrNoActiveOrg
	}
	return claims.ActiveOrganizationID, nil
}
