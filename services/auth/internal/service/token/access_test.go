package token

import (
	"testing"

	"github.com/golang-jwt/jwt/v5"

	"github.com/LegationPro/zagforge/shared/go/authclaims"
)

func TestIssueAccessToken_validClaims(t *testing.T) {
	svc := newTestService(t)

	tokenStr, err := svc.IssueAccessToken(AccessTokenParams{
		UserID: "550e8400-e29b-41d4-a716-446655440000",
		Email:  "user@example.com",
		Name:   "Jane Doe",
		Org: authclaims.OrgClaim{
			ID:   "org-uuid",
			Slug: "acme",
			Role: "admin",
		},
		Teams: []string{"team-1"},
		MFA:   true,
	})
	if err != nil {
		t.Fatalf("issue access token: %v", err)
	}

	if tokenStr == "" {
		t.Fatal("expected non-empty token")
	}

	// Parse and verify claims.
	claims := &authclaims.Claims{}
	parsed, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		return svc.publicKey, nil
	})
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}
	if !parsed.Valid {
		t.Fatal("expected valid token")
	}

	if claims.Subject != "550e8400-e29b-41d4-a716-446655440000" {
		t.Errorf("expected subject %q, got %q", "550e8400-e29b-41d4-a716-446655440000", claims.Subject)
	}
	if claims.Email != "user@example.com" {
		t.Errorf("expected email %q, got %q", "user@example.com", claims.Email)
	}
	if claims.Name != "Jane Doe" {
		t.Errorf("expected name %q, got %q", "Jane Doe", claims.Name)
	}
	if claims.Org.Slug != "acme" {
		t.Errorf("expected org slug %q, got %q", "acme", claims.Org.Slug)
	}
	if claims.Org.Role != "admin" {
		t.Errorf("expected org role %q, got %q", "admin", claims.Org.Role)
	}
	if !claims.MFA {
		t.Error("expected MFA true")
	}
	if len(claims.Teams) != 1 || claims.Teams[0] != "team-1" {
		t.Errorf("expected teams [team-1], got %v", claims.Teams)
	}
	if claims.Issuer != "test-issuer" {
		t.Errorf("expected issuer %q, got %q", "test-issuer", claims.Issuer)
	}
}

func TestIssueAccessToken_differentKeys_failsVerification(t *testing.T) {
	svc := newTestService(t)
	otherSvc := newTestService(t) // different key pair

	tokenStr, err := svc.IssueAccessToken(AccessTokenParams{
		UserID: "user-1",
		Email:  "a@b.com",
	})
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	// Verify with wrong key should fail.
	_, err = jwt.ParseWithClaims(tokenStr, &authclaims.Claims{}, func(t *jwt.Token) (any, error) {
		return otherSvc.publicKey, nil
	})
	if err == nil {
		t.Fatal("expected verification failure with different key")
	}
}
