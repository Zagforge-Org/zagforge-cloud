package github

import (
	"testing"

	"github.com/LegationPro/zagforge/auth/internal/service/oauth"
)

func TestNew_implementsProvider(t *testing.T) {
	var _ oauth.Provider = New("client-id", "client-secret", "http://localhost:8081")
}

func TestAuthURL_containsState(t *testing.T) {
	p := New("client-id", "client-secret", "http://localhost:8081")
	url := p.AuthURL("test-state-123")

	if url == "" {
		t.Fatal("expected non-empty auth URL")
	}

	// Should contain GitHub endpoint and state param.
	tests := []struct {
		name     string
		contains string
	}{
		{"github domain", "github.com"},
		{"client_id", "client_id=client-id"},
		{"state", "state=test-state-123"},
		{"redirect_uri", "redirect_uri="},
	}
	for _, tt := range tests {
		if !containsSubstring(url, tt.contains) {
			t.Errorf("auth URL missing %s: %s", tt.name, url)
		}
	}
}

func TestCallbackPath(t *testing.T) {
	if callbackPath != "/auth/oauth/github/callback" {
		t.Errorf("unexpected callback path: %s", callbackPath)
	}
}

func TestDefaultScopes(t *testing.T) {
	if len(defaultScopes) != 2 {
		t.Fatalf("expected 2 scopes, got %d", len(defaultScopes))
	}
	if defaultScopes[0] != ScopeReadUser {
		t.Errorf("expected first scope %q, got %q", ScopeReadUser, defaultScopes[0])
	}
	if defaultScopes[1] != ScopeUserEmail {
		t.Errorf("expected second scope %q, got %q", ScopeUserEmail, defaultScopes[1])
	}
}

func TestEndpoints(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		prefix   string
	}{
		{"user endpoint", userEndpoint, apiBaseURL},
		{"email endpoint", emailEndpoint, apiBaseURL},
	}
	for _, tt := range tests {
		if !containsSubstring(tt.endpoint, tt.prefix) {
			t.Errorf("%s should start with %s: %s", tt.name, tt.prefix, tt.endpoint)
		}
	}
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
