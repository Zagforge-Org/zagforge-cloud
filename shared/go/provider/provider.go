package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// ErrInvalidSignature is returned by ValidateWebhook when the HMAC signature does not match.
var ErrInvalidSignature = errors.New("invalid webhook signature")

type ActionType string

// WebhookEvent is the parsed result of a validated webhook payload.
type WebhookEvent struct {
	EventType      string // value of X-GitHub-Event header
	Action         ActionType
	RepoID         int64
	RepoName       string // "owner/repo"
	CloneURL       string // HTTPS clone URL from the payload
	Branch         string
	CommitSHA      string
	InstallationID int64
}

type Repo struct {
	ID            int64
	FullName      string
	DefaultBranch string
}

// PushPayload is the minimal GitHub webhook payload structure we need.
type PushPayload struct {
	Ref    string `json:"ref"`
	After  string `json:"after"`
	Action string `json:"action"`
	Repository struct {
		ID       int64  `json:"id"`
		FullName string `json:"full_name"`
		CloneURL string `json:"clone_url"`
	} `json:"repository"`
	Installation struct {
		ID int64 `json:"id"`
	} `json:"installation"`
}

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

// WebhookValidator is the minimal interface required by consumers that only validate webhooks.
type WebhookValidator interface {
	ValidateWebhook(ctx context.Context, payload []byte, signature string, eventType string) (WebhookEvent, error)
}

// Worker is the full provider interface.
type Worker interface {
	WebhookValidator
	GenerateCloneToken(ctx context.Context, installationID int64) (string, error)
	CloneRepo(ctx context.Context, repoURL, ref, token, dst string) error
	ListRepos(ctx context.Context, installationID int64) ([]Repo, error)
}
