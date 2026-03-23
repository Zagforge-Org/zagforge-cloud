package auth_test

import (
	"testing"

	"github.com/LegationPro/zagforge/api/internal/middleware/auth"
	"github.com/clerk/clerk-sdk-go/v2"
)

func TestResolveOrgIDMissingOrg(t *testing.T) {
	claims := &clerk.SessionClaims{} // no active org
	_, err := auth.ResolveClerkOrgID(claims)
	if err == nil {
		t.Fatal("expected error when no active org in claims")
	}
}
