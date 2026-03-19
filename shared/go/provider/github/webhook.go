package github

import "context"

// Declare ActionType as a string. Will be used for comparing the action field in the webhook payload.
type ActionType string

// WebhookEvent is the parsed result of a validated webhook payload.
type WebhookEvent struct {
	EventType      string // value of X-GitHub-Event header
	Action         ActionType
	RepoID         int64
	RepoName       string
	CloneURL       string
	Branch         string
	CommitSHA      string
	InstallationID int64
}

// WebhookValidator is the minimal interface required by consumers that only validate webhooks.
type WebhookValidator interface {
	ValidateWebhook(ctx context.Context, payload []byte, signature string, eventType string) (WebhookEvent, error)
}
