package token

import (
	"testing"
	"time"
)

func TestMFAChallengeToken_roundtrip(t *testing.T) {
	svc := newTestService(t)
	userID := "550e8400-e29b-41d4-a716-446655440000"

	tokenStr, err := svc.IssueMFAChallengeToken(userID)
	if err != nil {
		t.Fatalf("issue mfa token: %v", err)
	}

	subject, err := svc.ValidateMFAChallengeToken(tokenStr)
	if err != nil {
		t.Fatalf("validate mfa token: %v", err)
	}
	if subject != userID {
		t.Errorf("expected subject %q, got %q", userID, subject)
	}
}

func TestMFAChallengeToken_wrongKey_fails(t *testing.T) {
	svc := newTestService(t)
	otherSvc := newTestService(t)

	tokenStr, err := svc.IssueMFAChallengeToken("user-1")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	_, err = otherSvc.ValidateMFAChallengeToken(tokenStr)
	if err == nil {
		t.Fatal("expected error validating with wrong key")
	}
}

func TestMFAChallengeToken_invalidString_fails(t *testing.T) {
	svc := newTestService(t)
	_, err := svc.ValidateMFAChallengeToken("not-a-valid-token")
	if err == nil {
		t.Fatal("expected error for invalid token string")
	}
}

func TestMFAChallengeToken_accessToken_failsAudienceCheck(t *testing.T) {
	svc := newTestService(t)

	// Issue a regular access token (no mfa_challenge audience).
	accessToken, err := svc.IssueAccessToken(AccessTokenParams{
		UserID: "user-1",
		Email:  "a@b.com",
	})
	if err != nil {
		t.Fatalf("issue access: %v", err)
	}

	// Should fail audience check.
	_, err = svc.ValidateMFAChallengeToken(accessToken)
	if err == nil {
		t.Fatal("expected error: access token should not pass as MFA challenge")
	}
}

func TestMFAChallengeToken_shortTTL(t *testing.T) {
	// Verify the TTL is 5 minutes by checking the constant.
	if mfaChallengeTTL != 5*time.Minute {
		t.Errorf("expected mfa challenge TTL 5m, got %v", mfaChallengeTTL)
	}
}
