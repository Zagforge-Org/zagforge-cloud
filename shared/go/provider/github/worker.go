package github

import "context"

// Worker is the full provider interface.
type Worker interface {
	WebhookValidator
	GenerateCloneToken(ctx context.Context, installationID int64) (string, error)
	CloneRepo(ctx context.Context, repoURL, ref, token, dst string) error
	ListRepos(ctx context.Context, installationID int64) ([]Repo, error)
	GetBlob(ctx context.Context, installationID int64, repoFullName, sha string) (string, error)
}
