# Zagforge — Data Model [Phase 1]

Migrations live at `api/db/migrations/` (owned by the API service). SQL queries at `api/db/queries/`, sqlc generates to `api/internal/db/`.

## `users`

Synced from Zitadel via OIDC JWT claims on first login and updated via Zitadel webhooks (Action events). Zitadel is the source of truth for authentication; this table caches identity data for relational queries and audit trails.

A user has their own personal workspace (repos, tokens, AI keys) and can optionally create or join organizations for team collaboration.

```sql
CREATE TABLE users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    zitadel_user_id TEXT UNIQUE NOT NULL,
    username        TEXT UNIQUE NOT NULL,
    email           TEXT UNIQUE NOT NULL,
    email_verified  BOOLEAN NOT NULL DEFAULT false,
    phone           TEXT,
    avatar_url      TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TRIGGER users_set_updated_at
BEFORE UPDATE ON users
FOR EACH ROW EXECUTE FUNCTION set_updated_at();
```

## `organizations`

Optional team workspaces. A user can create and own multiple organizations, and can be a member of organizations owned by others. Organizations are managed via the Zagforge API (with Zitadel handling the auth layer).

```sql
CREATE TABLE organizations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    zitadel_org_id  TEXT UNIQUE NOT NULL,
    slug            TEXT UNIQUE NOT NULL,     -- URL-safe identifier for API paths
    name            TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_organizations_slug ON organizations (slug);
```

## `memberships`

Tracks user-to-org relationships with roles. A user can be a member of many organizations. The `owner` role is assigned to the user who created the org.

```sql
CREATE TABLE memberships (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    org_id     UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    role       TEXT NOT NULL DEFAULT 'member'
               CHECK (role IN ('owner', 'admin', 'member')),
    invited_by UUID REFERENCES users(id),
    joined_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, org_id)
);

CREATE INDEX idx_memberships_user_id ON memberships (user_id);
CREATE INDEX idx_memberships_org_id ON memberships (org_id);
```

## `sessions`

Tracks active user sessions across devices. Synced from Zitadel session events. Users can view and revoke sessions from the dashboard.

```sql
CREATE TABLE sessions (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id            UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    zitadel_session_id TEXT UNIQUE NOT NULL,
    device_name        TEXT,          -- e.g. "Chrome on macOS", "iPhone 15"
    ip_address         INET,
    last_active_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_sessions_user_id ON sessions (user_id);
```

## `audit_log`

Append-only log of mutations within an org or a user's personal workspace. Surfaced to org owners/admins in the dashboard. For personal workspace actions, `org_id` is NULL and `user_id` identifies the owner.

```sql
CREATE TABLE audit_log (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID REFERENCES users(id),              -- personal workspace scope
    org_id      UUID REFERENCES organizations(id) ON DELETE CASCADE,  -- org scope
    actor_id    UUID NOT NULL REFERENCES users(id),
    action      TEXT NOT NULL,   -- e.g. 'member.invited', 'member.removed', 'member.role_changed', 'org.updated', 'org.deleted', 'repo.connected', 'token.created'
    target_id   UUID,            -- the user/resource acted upon
    metadata    JSONB,           -- e.g. { "role_from": "member", "role_to": "admin" }
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (
        (user_id IS NOT NULL AND org_id IS NULL) OR
        (user_id IS NULL AND org_id IS NOT NULL)
    )
);

CREATE INDEX idx_audit_log_org_id ON audit_log (org_id, created_at DESC);
CREATE INDEX idx_audit_log_user_id ON audit_log (user_id, created_at DESC);
```

---

## Resource Ownership Model

All resource tables (repositories, ai_provider_keys, context_tokens, cli_api_keys) support dual ownership: **personal** (owned by a user) or **organizational** (owned by an org). Exactly one must be set.

```sql
-- Pattern applied to every resource table:
user_id  UUID REFERENCES users(id) ON DELETE CASCADE,
org_id   UUID REFERENCES organizations(id) ON DELETE CASCADE,
CHECK (
    (user_id IS NOT NULL AND org_id IS NULL) OR
    (user_id IS NULL AND org_id IS NOT NULL)
)
```

The auth middleware resolves the active scope (personal vs. org) from the request path or JWT claims, then all downstream queries filter by the correct column. This lets solo developers use Zagforge without creating an organization, while teams get full multi-tenant isolation.

## `repositories`

```sql
CREATE TABLE repositories (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID REFERENCES users(id) ON DELETE CASCADE,
    org_id          UUID REFERENCES organizations(id) ON DELETE CASCADE,
    github_repo_id  BIGINT UNIQUE NOT NULL,
    installation_id BIGINT NOT NULL,         -- GitHub App installation ID (for generating IATs)
    full_name       TEXT NOT NULL,          -- e.g. "LegationPro/zigzag"
    default_branch  TEXT NOT NULL DEFAULT 'main',
    installed_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (
        (user_id IS NOT NULL AND org_id IS NULL) OR
        (user_id IS NULL AND org_id IS NOT NULL)
    )
);

CREATE INDEX idx_repositories_user_id ON repositories (user_id);
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
    user_id     UUID REFERENCES users(id) ON DELETE CASCADE,
    org_id      UUID REFERENCES organizations(id) ON DELETE CASCADE,
    provider    TEXT NOT NULL CHECK (provider IN ('openai', 'anthropic', 'google', 'xai')),
    key_cipher  BYTEA NOT NULL,   -- nonce (12 bytes) || AES-256-GCM ciphertext
    key_hint    TEXT NOT NULL,    -- last 4 chars for UI display e.g. "...xK9f"
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (COALESCE(user_id, '00000000-0000-0000-0000-000000000000'),
            COALESCE(org_id,  '00000000-0000-0000-0000-000000000000'),
            provider),
    CHECK (
        (user_id IS NOT NULL AND org_id IS NULL) OR
        (user_id IS NULL AND org_id IS NOT NULL)
    )
);
```

Encryption key sourced from Google Secret Manager at runtime. Never stored in env vars or Go source.

### New: `context_tokens`

Backing table for the Context URL feature. Each token is a revocable, optionally-expiring credential scoped to a single repo.

```sql
CREATE TABLE context_tokens (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    repo_id            UUID NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    user_id            UUID REFERENCES users(id) ON DELETE CASCADE,
    org_id             UUID REFERENCES organizations(id) ON DELETE CASCADE,
    target_snapshot_id UUID REFERENCES snapshots(id) ON DELETE SET NULL,
    -- NULL = always resolve to latest snapshot
    -- SET  = locked to a specific snapshot version (stable docs, release tags, etc.)
    token_hash         TEXT UNIQUE NOT NULL,  -- SHA-256(raw_token); raw token never stored
    label              TEXT,                  -- user-supplied e.g. "Cursor Rules", "Claude Project"
    last_used_at       TIMESTAMPTZ,           -- updated async, not on the critical path
    expires_at         TIMESTAMPTZ,           -- NULL = never expires
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (
        (user_id IS NOT NULL AND org_id IS NULL) OR
        (user_id IS NULL AND org_id IS NOT NULL)
    )
);

CREATE INDEX idx_context_tokens_hash ON context_tokens (token_hash);
```

**Token format:** `zf_ctx_<32 random URL-safe chars>` (~192 bits entropy). Raw token returned once at creation time; only the SHA-256 hash is persisted. Format prefix enables GitHub secret scanning integration.
