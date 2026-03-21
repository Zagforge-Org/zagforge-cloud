# Zagforge — Data Model [Phase 1]

Migrations live at `api/db/migrations/` (owned by the API service). SQL queries at `api/db/queries/`, sqlc generates to `api/internal/db/`.

## `organizations`

Managed by Clerk. The Go API syncs org metadata via Clerk webhooks.

```sql
CREATE TABLE organizations (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    clerk_org_id TEXT UNIQUE NOT NULL,
    slug         TEXT UNIQUE NOT NULL,     -- URL-safe identifier for API paths
    name         TEXT NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_organizations_slug ON organizations (slug);
```

## `repositories`

```sql
CREATE TABLE repositories (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES organizations(id),
    github_repo_id  BIGINT UNIQUE NOT NULL,
    installation_id BIGINT NOT NULL,         -- GitHub App installation ID (for generating IATs)
    full_name       TEXT NOT NULL,          -- e.g. "LegationPro/zigzag"
    default_branch  TEXT NOT NULL DEFAULT 'main',
    installed_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_repositories_org_id ON repositories (org_id);
CREATE INDEX idx_repositories_full_name ON repositories (full_name);
```

**Repo rename handling:** The webhook handler also listens for `repository.renamed` events and updates `full_name` accordingly. This keeps API paths (`/api/v1/{org}/{repo}/...`) functional after renames.

Note: Webhook HMAC validation uses the **GitHub App-level webhook secret** (a single secret configured on the GitHub App, stored in Secret Manager), not per-repo secrets. This is because GitHub sends all webhooks for an App to the same endpoint, and the HMAC must be validated before the payload can be trusted to extract a repo ID.

## `jobs`

```sql
CREATE TABLE jobs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    repo_id         UUID NOT NULL REFERENCES repositories(id),
    branch          TEXT NOT NULL,
    commit_sha      TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'queued'
                    CHECK (status IN ('queued', 'running', 'succeeded', 'failed', 'cancelled')),
    error_message   TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at      TIMESTAMPTZ,
    finished_at     TIMESTAMPTZ
);

-- Partial index for fast dedup lookups
CREATE INDEX idx_jobs_dedup ON jobs (repo_id, branch, status)
    WHERE status IN ('queued', 'running');

-- Index for timeout watchdog queries
CREATE INDEX idx_jobs_running ON jobs (status, started_at)
    WHERE status = 'running';
```

## `snapshots`

```sql
CREATE TABLE snapshots (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    repo_id          UUID NOT NULL REFERENCES repositories(id),
    job_id           UUID NOT NULL REFERENCES jobs(id),
    branch           TEXT NOT NULL,
    commit_sha       TEXT NOT NULL,
    gcs_path         TEXT NOT NULL,
    snapshot_version INT NOT NULL DEFAULT 1,
    zigzag_version   TEXT NOT NULL,
    size_bytes       BIGINT NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_snapshots_latest ON snapshots (repo_id, branch, created_at DESC);
CREATE INDEX idx_snapshots_job_id ON snapshots (job_id);
CREATE UNIQUE INDEX idx_snapshots_unique ON snapshots (repo_id, branch, commit_sha);
```

---

## Phase 5 Additions

### Migration: `snapshots` — make `job_id` nullable, add `metadata`

```sql
-- CLI-uploaded snapshots have no associated job; job_id is only set by the worker
ALTER TABLE snapshots ALTER COLUMN job_id DROP NOT NULL;

-- Stores ignore_patterns and zigzag_config_hash from the Zigzag run
-- Used by the Context Proxy to know which files to exclude when fetching from GitHub
ALTER TABLE snapshots ADD COLUMN metadata JSONB;
```

`job_id` is NULL for CLI-sourced snapshots; populated for webhook-triggered snapshots. Query paths that reference jobs must handle NULL.

### New: `ai_provider_keys`

Stores encrypted AI provider credentials per org. Used by the Query Console to proxy requests to OpenAI, Anthropic, or Google.

```sql
CREATE TABLE ai_provider_keys (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id      UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    provider    TEXT NOT NULL CHECK (provider IN ('openai', 'anthropic', 'google')),
    key_cipher  BYTEA NOT NULL,   -- nonce (12 bytes) || AES-256-GCM ciphertext
    key_hint    TEXT NOT NULL,    -- last 4 chars for UI display e.g. "...xK9f"
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (org_id, provider)
);
```

Encryption key sourced from Google Secret Manager at runtime. Never stored in env vars or Go source.

### New: `context_tokens`

Backing table for the Context URL feature. Each token is a revocable, optionally-expiring credential scoped to a single repo.

```sql
CREATE TABLE context_tokens (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    repo_id            UUID NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    org_id             UUID NOT NULL REFERENCES organizations(id),
    target_snapshot_id UUID REFERENCES snapshots(id) ON DELETE SET NULL,
    -- NULL = always resolve to latest snapshot
    -- SET  = locked to a specific snapshot version (stable docs, release tags, etc.)
    token_hash         TEXT UNIQUE NOT NULL,  -- SHA-256(raw_token); raw token never stored
    label              TEXT,                  -- user-supplied e.g. "Cursor Rules", "Claude Project"
    last_used_at       TIMESTAMPTZ,           -- updated async, not on the critical path
    expires_at         TIMESTAMPTZ,           -- NULL = never expires
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_context_tokens_hash ON context_tokens (token_hash);
```

**Token format:** `zf_ctx_<32 random URL-safe chars>` (~192 bits entropy). Raw token returned once at creation time; only the SHA-256 hash is persisted. Format prefix enables GitHub secret scanning integration.
