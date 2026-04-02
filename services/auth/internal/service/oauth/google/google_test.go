package google

import (
	"strings"
	"testing"

	"github.com/LegationPro/zagforge/auth/internal/service/oauth"
)

func TestNew_implementsProvider(t *testing.T) {
	var _ oauth.Provider = New("client-id", "client-secret", "http://localhost:8081")
}

func TestAuthURL_containsState(t *testing.T) {
	p := New("client-id", "client-secret", "http://localhost:8081")
	url := p.AuthURL("test-state-456")

	if url == "" {
		t.Fatal("expected non-empty auth URL")
	}

	tests := []struct {
		name     string
		contains string
	}{
		{"google domain", "accounts.google.com"},
		{"client_id", "client_id=client-id"},
		{"state", "state=test-state-456"},
		{"access_type offline", "access_type=offline"},
	}
	for _, tt := range tests {
		if !strings.Contains(url, tt.contains) {
			t.Errorf("auth URL missing %s: %s", tt.name, url)
		}
	}
}

func TestCallbackPath(t *testing.T) {
	if callbackPath != "/auth/oauth/google/callback" {
		t.Errorf("unexpected callback path: %s", callbackPath)
	}
}

func TestDefaultScopes(t *testing.T) {
	if len(defaultScopes) != 3 {
		t.Fatalf("expected 3 scopes, got %d", len(defaultScopes))
	}
	expected := []string{ScopeOpenID, ScopeEmail, ScopeProfile}
	for i, s := range expected {
		if defaultScopes[i] != s {
			t.Errorf("scope[%d]: expected %q, got %q", i, s, defaultScopes[i])
		}
	}
}

func TestUserInfoEndpoint(t *testing.T) {
	if !strings.Contains(userInfoEndpoint, "googleapis.com") {
		t.Errorf("unexpected userinfo endpoint: %s", userInfoEndpoint)
	}
}
