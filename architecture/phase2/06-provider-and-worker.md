# Zagforge — Provider Interface & Worker [Phase 2]

The GitHub implementation lives in `shared/go/provider/github/` as a **concrete struct**. Consumers (API handlers, worker runner) define their own small interfaces that the struct satisfies — following Go's "accept interfaces, return structs" idiom.

## Provider Types (shared/go/provider/)

Shared types used across consumers. No interface here — just the data structures and the concrete GitHub implementation:

```go
package provider

// WebhookEvent is the parsed result of a validated webhook payload.
type WebhookEvent struct {
    Action    string // "push", "installation", "repository.renamed", etc.
    RepoID    int64
    RepoName  string // "org/repo"
    Branch    string
    CommitSHA string
}

// Repo represents a repository accessible via a provider.
type Repo struct {
    ID            int64
    FullName      string
    DefaultBranch string
}
```

## GitHub Implementation (shared/go/provider/github/)

A concrete struct that exposes all provider operations. Consumers pick what they need via their own interfaces:

```go
package github

// Client is the concrete GitHub App client.
// It satisfies whatever small interface each consumer defines.
type Client struct {
    appID      int64
    privateKey []byte
}

func NewClient(appID int64, privateKey []byte) *Client {
    return &Client{appID: appID, privateKey: privateKey}
}

// ValidateWebhook checks HMAC-SHA256 signature and parses the payload.
// Takes raw bytes + signature string — no coupling to net/http.
func (c *Client) ValidateWebhook(ctx context.Context, payload []byte, signature string, secret string) (provider.WebhookEvent, error)

// GenerateCloneToken creates a short-lived GitHub Installation Access Token (IAT).
func (c *Client) GenerateCloneToken(ctx context.Context, installationID int64) (string, error)

// CloneRepo performs a shallow clone of the repo at the given ref.
func (c *Client) CloneRepo(ctx context.Context, repoURL string, ref string, token string, dst string) error

// ListRepos returns repos accessible to the authenticated installation.
func (c *Client) ListRepos(ctx context.Context, installationID int64) ([]provider.Repo, error)
```

## Consumer Interfaces (defined at call site)

Each consumer defines only the interface it needs. The `github.Client` satisfies both without knowing about them:

```go
// In api/internal/handler/webhooks.go
type WebhookValidator interface {
    ValidateWebhook(ctx context.Context, payload []byte, signature string, secret string) (provider.WebhookEvent, error)
}

// In api/internal/handler/snapshots.go (for listing repos on install)
type RepoLister interface {
    ListRepos(ctx context.Context, installationID int64) ([]provider.Repo, error)
}

// In worker/internal/runner/runner.go
type RepoCloner interface {
    GenerateCloneToken(ctx context.Context, installationID int64) (string, error)
    CloneRepo(ctx context.Context, repoURL string, ref string, token string, dst string) error
}
```

This makes each component testable with minimal mocks — you only stub the methods that component actually calls.

---

## Worker Container

The Cloud Run Job container is minimal:

1. Read job config from environment variables (`JOB_ID`, `REPO_URL`, `COMMIT_SHA`, `BRANCH`, `GCS_BUCKET`, `CALLBACK_URL`, `JOB_TOKEN`)
2. `POST /internal/jobs/start` with signed job token — response returns latest `commit_sha` and a short-lived GitHub Installation Access Token (IAT) for cloning private repos
3. `git clone --depth 1 --branch $BRANCH https://x-access-token:$IAT@github.com/$REPO /workspace`
4. Run Zigzag binary on `/workspace`
5. Upload `snapshot.json` to GCS
6. `POST /internal/jobs/complete` with result metadata
7. Exit

The production container image includes: worker binary, Zigzag binary (installed via a dedicated Docker build stage), and git. See `09-docker.md` for the multi-stage Dockerfile.

**Dev mode:** Set `ZIGZAG_MOCK=true` to skip actual Zigzag execution and generate a stub snapshot. This allows testing the full webhook→job→callback flow without a real repo clone. Alternatively, set `ZIGZAG_BIN` to point to a locally mounted Zigzag binary.

On any error at steps 3-5, the worker calls `/internal/jobs/complete` with `status: "failed"` and the error message.

**Cloud Run Job configuration:**
- CPU: 2 vCPU
- Memory: 4 GiB
- Timeout: 15 minutes
- Max retries: 0 (retries handled at Cloud Tasks level)
