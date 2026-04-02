package github

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// ErrInvalidSignature is returned by ValidateWebhook when the HMAC signature does not match.
var ErrInvalidSignature = errors.New("invalid webhook signature")

// ParsePayload parses a Github webhook JSON payload into a PushPayload go struct.
func ParsePayload(payload []byte) (PushPayload, error) {
	var p PushPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return PushPayload{}, err
	}
	return p, nil
}

// BranchFromRef strips the "refs/heads/" prefix from a Git ref.
// If the ref is not a branch ref, it is returned as-is.
func BranchFromRef(ref string) string {
	return strings.TrimPrefix(ref, "refs/heads/")
}

// BuildAuthURL injects an installation access token into an HTTPS repo URL.
// For file:// URLs (e.g. in tests) the token is ignored and the URL is returned unchanged.
func BuildAuthURL(repoURL, token string) (string, error) {
	u, err := url.Parse(repoURL)
	if err != nil {
		return "", fmt.Errorf("invalid repo URL: %w", err)
	}
	if token != "" && u.Scheme == "https" {
		u.User = url.UserPassword("x-access-token", token)
	}
	return u.String(), nil
}
