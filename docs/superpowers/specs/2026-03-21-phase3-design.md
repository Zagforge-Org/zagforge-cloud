# Phase 3 Design Spec — Zagforge Context Proxy & Dashboard

**Date:** 2026-03-21
**Status:** Approved
**Scope:** Dashboard (cloud.zagforge.com) + Context Proxy backend + CLI upload bridge

---

## 1. Problem & Goal

Zigzag (the CLI) solves extraction — it turns a codebase into structured markdown. The remaining friction is **distribution**: developers manually re-upload a 500KB file into a chat window every time the code changes.

Phase 3 is done when Zagforge becomes a **Context Proxy**: a persistent URL that any AI tool can fetch to get the latest, accurate snapshot of a codebase — without the developer touching anything after the first upload.

**The "Aha!" moment:** A developer pushes their snapshot once. They paste one URL into Cursor Rules or a Claude Project. From that point on, their AI is always in sync with their code.

---

## 2. Decisions Log

| Decision | Choice | Rationale |
|----------|--------|-----------|
| MVP feature | Context URL (GET endpoint returning markdown) | Ships real value with zero AI infrastructure; chat UI is the upsell |
| AI execution | Server-side proxy (Go) | Enables caching, token optimization, observability; client-side loses the moat |
| Code at rest | Never stored | Only metadata (file map + SHAs) in GCS; code fetched from GitHub ephemerally at query time |
| First ingestion | `zigzag --upload` (CLI) | Proves demand before automating; GitHub webhook automation is Phase 3b |
| CLI placement | Open source main repo | Trust + transparency; `--upload` visible in `--help` drives cloud adoption |
| Auth | Clerk headless (Next.js hooks) | Go backend unchanged; full UI control; zero migration risk |
| Multi-tenancy | Clerk Orgs + DB org_id now, billing Phase 4 | Avoids schema migration debt; Clerk handles invite emails |
| Analytics | Phase 4 | Data stays in DB; no API/UI build yet |
| System prompt | Hardcoded on backend | "Just works" for Phase 3; user customization is Phase 4 |

---

## 3. Architecture

```
Developer Machine
  zigzag --upload
    → POST /api/v1/upload (metadata JSON only, no code)
    → Auth: Clerk API key

Go API (api.zagforge.com)
  POST /api/v1/upload              stores metadata snapshot in GCS
  HEAD /v1/context/{token}         token lookup only, returns X-Snapshot-ID, X-Commit-SHA
  GET  /v1/context/{token}         Context URL — assembles + streams report.llm.md
  POST /api/v1/{org}/{repo}/query  Query Console — SSE stream to AI provider
  CRUD /api/v1/{org}/{repo}/context-tokens
  CRUD /api/v1/{org}/settings/ai-keys

GCS Bucket (zagforge-snapshots)
  Stores: metadata snapshot only (file tree + SHAs, no content)
  Never stores: file contents

GitHub App
  Used at query time: fetches file contents via installation token
  Contents assembled in memory, never written to disk or cache disk

Redis LRU Cache
  Key: ctx:{repo_id}:{commit_sha}   ← namespaced to avoid collision with rate limiter keys
  Value: fully assembled report.llm.md (in-memory string)
  TTL: 10 minutes
  Purpose: avoids redundant GitHub API calls within a session
  Note: last_used_at writes are batched async (fire-and-forget goroutine) to avoid
        write-on-read latency on high-traffic context URLs

Next.js Dashboard (cloud.zagforge.com)
  apps/cloud in zigzag-web monorepo
  Auth pages, repo list, Context URL panel, Query Console, settings
```

**Zero-trust guarantee:** File contents are ephemeral. They exist only in Go process memory during assembly. They are never written to GCS, Postgres, or any persistent store.

---

## 4. Data Model Changes

### 4a. Snapshot format change (`snapshot_version: 2`)

GCS `snapshot.json` drops `files[].content`. Replaces with `file_tree`:

```json
{
  "snapshot_version": 2,
  "zigzag_version": "0.12.0",
  "commit_sha": "3fa912e...",
  "branch": "main",
  "generated_at": "2026-03-21T12:00:00Z",
  "summary": {
    "source_files": 42,
    "total_lines": 8500,
    "total_size_bytes": 245000,
    "languages": [{ "name": "go", "files": 30, "lines": 6200 }]
  },
  "file_tree": [
    { "path": "cmd/api/main.go", "language": "go", "lines": 87, "sha": "abc123" }
  ]
}
```

### 4b. `snapshots` table — migration (two changes)

```sql
-- Make job_id nullable: CLI uploads have no associated job
ALTER TABLE snapshots ALTER COLUMN job_id DROP NOT NULL;

-- Add metadata for ignore rules used during Zigzag run
ALTER TABLE snapshots ADD COLUMN metadata JSONB;
```

`job_id` is NULL for CLI-uploaded snapshots; set by the worker for GitHub webhook-triggered snapshots. The Context Proxy must handle both cases.

`metadata` stores: `ignore_patterns` (from `.zigzagignore`), `zigzag_config_hash`. Used by the Context Proxy to know which files to exclude when fetching from GitHub.

**v1 snapshot compatibility:** If the Context Proxy resolves a `snapshot_version: 1` snapshot (missing `file_tree`), it returns HTTP 422 with body `{ "error": "snapshot_outdated", "message": "Re-run zigzag --upload to generate a v2 snapshot." }`. No silent failures.

### 4c. New: `ai_provider_keys`

```sql
CREATE TABLE ai_provider_keys (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id      UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    provider    TEXT NOT NULL CHECK (provider IN ('openai', 'anthropic', 'google')),
    key_cipher  BYTEA NOT NULL,   -- 12-byte GCM nonce || ciphertext, key from Secret Manager
    key_hint    TEXT NOT NULL,    -- last 4 chars e.g. "...xK9f" for UI display
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (org_id, provider)
);
```

Encryption: AES-256-GCM. `key_cipher` stores `nonce (12 bytes) || ciphertext`. Encryption key sourced from Secret Manager, never in Go source or env vars.

### 4d. New: `context_tokens`

```sql
CREATE TABLE context_tokens (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    repo_id            UUID NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    org_id             UUID NOT NULL REFERENCES organizations(id),
    target_snapshot_id UUID REFERENCES snapshots(id) ON DELETE SET NULL,
    -- NULL = always latest; SET = locked to a specific snapshot version
    token_hash         TEXT UNIQUE NOT NULL,  -- SHA-256(raw_token), never store raw
    label              TEXT,                  -- e.g. "Cursor Rules", "Claude Project"
    last_used_at       TIMESTAMPTZ,
    expires_at         TIMESTAMPTZ,           -- NULL = never expires
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_context_tokens_hash ON context_tokens (token_hash);
```

**Token format:** `zf_ctx_<32 random URL-safe chars>` (minimum 32 chars = ~192 bits entropy; brute-force infeasible). Raw token returned once at creation, never stored. Enables GitHub secret scanning integration.

---

## 5. API Endpoints (Phase 3 additions)

All existing endpoints unchanged.

### CLI Upload
```
POST /api/v1/upload
Auth: Zagforge CLI token (Authorization: Bearer zf_pk_<token>)
      NOT a Clerk session JWT. CLI tokens are long-lived API keys issued
      by Zagforge (stored hashed in a new `api_keys` table, or via Clerk
      machine tokens — implementation choice, but must be a distinct
      middleware from the Clerk JWT session middleware).
Body: {
  org_slug: string,           // Clerk org slug
  repo_full_name: string,     // "github-owner/repo-name"
  commit_sha: string,
  branch: string,
  metadata_snapshot: object   // snapshot_version 2 JSON
}

Backend logic:
  1. Verify bearer token via CLI token middleware (separate from Clerk JWT middleware)
  2. Resolve org_slug → org_id; verify token's org matches request org (no cross-org writes)
  3. Look up repositories row by repo_full_name + org_id
     → If not found AND no GitHub App installation exists: return 404
       { "error": "repo_not_connected", "message": "Install the Zagforge GitHub App first." }
     → If not found but installation_id is known: auto-create repositories row
  4. Insert snapshot row (job_id = NULL for CLI uploads)
  5. Write snapshot.json to GCS

Response: { snapshot_id: uuid, created_at: timestamp }
```

### Context URL
```
HEAD /v1/context/{raw_token}
  → SHA-256(raw_token) lookup
  → Token not found: 404 (same as GET)
  → Token expired: 410 Gone (same as GET)
  → Returns 200 OK with headers:
      X-Snapshot-ID: <uuid>
      X-Commit-SHA: <sha>
      Content-Type: text/markdown
  → No GitHub fetch triggered

GET /v1/context/{raw_token}?format=markdown|json
  → SHA-256(raw_token) lookup
  → Token not found: 404
  → Token expired (expires_at < now): 410 Gone (not 401/403 — avoids oracle attack)
  → Update last_used_at async (fire-and-forget, not on critical path)
  → Resolve snapshot (target_snapshot_id or latest)
  → Snapshot is v1 (no file_tree): 422 with upgrade message (see Section 4b)
  → Check Redis LRU (key: ctx:{repo_id}:{commit_sha})
  → Cache hit: stream immediately
  → Cache miss: streaming assembly pipeline:
      1. Flush markdown header immediately (zero perceived latency)
      2. For each file in file_tree: fetch from GitHub, wrap in code block, flush
      3. Store assembled result in Redis (TTL 10 min)
  → Content-Type: text/markdown (or application/json)
  → CORS: Access-Control-Allow-Origin: * (callers are AI tools, not browser origins)
  → Rate limit: 60 req/min per token_hash (Redis key: rl:ctx:{token_hash}, separate namespace from /api/v1/ rate limiter keys)
```

### Query Console
```
POST /api/v1/{org}/{repo}/query
Auth: Clerk JWT
Body: { question: string, snapshot_id?: uuid }
Response: text/event-stream (SSE)

Route parameter note: {org} is the Clerk org slug (e.g. "zagforge"),
{repo} is the repository slug derived from full_name suffix (e.g. "zigzag"
for "LegationPro/zigzag"). These match the existing /api/v1/{org}/{repo}/...
pattern already in the backend.

Backend logic:
  1. Load snapshot (snapshot_id or latest)
  2. If snapshot is v1: return 422 (see Section 4b)
  3. Assemble context (Redis LRU key ctx:{repo_id}:{commit_sha} or GitHub fetch)
  4. Select AI provider: use first available key in order Anthropic → OpenAI → Google
     If no key configured: return 422 { "error": "no_ai_key",
     "message": "Add an AI provider key in Settings to use the Query Console." }
  5. Decrypt selected key (AES-256-GCM + Secret Manager)
  6. Prepend hardcoded system prompt:
     "You are an expert on this codebase. Refer to files by their full path.
      Answer concisely and precisely. If unsure, say so."
  7. Stream provider response via SSE → browser
```

### Context Token Management
```
GET    /api/v1/{org}/{repo}/context-tokens       list (label, hint, last_used_at, expires_at)
POST   /api/v1/{org}/{repo}/context-tokens       create → returns raw token ONCE in response
DELETE /api/v1/{org}/{repo}/context-tokens/{id}  revoke (delete row)
```

### AI Key Management
```
PUT    /api/v1/{org}/settings/ai-keys            { provider, raw_key } → encrypt + store
DELETE /api/v1/{org}/settings/ai-keys/{provider} remove
GET    /api/v1/{org}/settings/ai-keys            list { provider, key_hint, created_at } only
```

---

## 6. CLI Changes (`zigzag` open source repo)

**Location:** `/internal/cloud/` package within the main zigzag repo.

**Flag:** `zigzag --upload` (opt-in; no behavior change without the flag).

**Auth:** `ZAGFORGE_API_KEY` env var. If missing, CLI prints:
```
Run `zigzag login` or set ZAGFORGE_API_KEY to use cloud features.
```

**What it sends:** Only the `snapshot_version: 2` JSON (file tree + SHAs + summary). No file contents leave the machine via this path.

**Isolation:** All upload logic lives in `/internal/cloud/`. Zero coupling to the core Zigzag engine. The flag is a thin wrapper that calls `cloud.Upload(snapshot, apiKey)`.

---

## 7. Next.js `apps/cloud` Structure

New app in `zigzag-web` monorepo. Uses existing `@workspace/ui` components, `ThemeProvider`, and `themeSyncScript`.

```
apps/cloud/
  app/
    (auth)/
      sign-in/page.tsx
      sign-up/page.tsx
      forgot-password/page.tsx
      verify/page.tsx              email verification step
      layout.tsx                   centered card, no sidebar

    (dashboard)/
      layout.tsx                   sidebar + OrganizationSwitcher + UserButton
      page.tsx                     redirect → /repos

      repos/
        page.tsx                   repo list; empty → onboarding-wizard
        [[...repo]]/
          page.tsx                 Context URL panel + token table + Query Console

      settings/
        ai-keys/page.tsx           add/remove provider keys (key_hint displayed)
        team/page.tsx              Clerk OrganizationSwitcher + member list
        billing/page.tsx           "Pro features coming soon — contact us" stub

    not-found.tsx                  custom 404

  features/
    auth/components/               sign-in-form, sign-up-form, forgot-password-form, verify-form
    repos/components/              repo-card, snapshot-badge, empty-state
    onboarding/components/         onboarding-wizard (quick start with code block)
    context/components/            context-url-panel, token-table, copy-button
    chat/components/               query-console, message-bubble, sse-stream-hook
    settings/components/           ai-key-form, key-hint-row, billing-stub

  providers/
    clerk-provider.tsx
    react-query-provider.tsx

  lib/
    api.ts                         typed fetch wrapper → api.zagforge.com
    sse.ts                         SSE hook for query streaming
```

### Deployment

`apps/cloud` is deployed as a separate Vercel project (same pattern as `apps/web` and `apps/docs`). Add to `vercel.json` at the monorepo root or configure via Vercel's monorepo support. Domain: `cloud.zagforge.com`. No changes to the Go API deployment or Terraform (Phase 3 Terraform work is separate).

### Key UX decisions

**`[[...repo]]` catch-all segment** — the URL is `cloud.zagforge.com/repos/{clerk-org-slug}/{repo-slug}` where `{clerk-org-slug}` is the Clerk organization slug (not the GitHub owner name — these may differ). The frontend resolves the Clerk org slug from the active session and passes it to API calls as `{org}`. The `{repo}` segment matches the repository's `full_name` suffix stored in the DB.

**Context URL panel is above the fold** on `[[...repo]]/page.tsx`. Two copy buttons:
- **Copy for Cursor** — URL + Cursor Rules snippet
- **Copy for Claude** — URL + "Fetch this URL for codebase context" instruction

**Query Console** is below the fold on the same page. Same repo, no navigation required.

**Onboarding wizard** shown when user has zero repos. Displays:
```bash
export ZAGFORGE_API_KEY=zf_pk_...

zigzag --upload
```
Turns empty state into a Quick Start guide.

**Theme sync** — `NEXT_PUBLIC_COOKIE_DOMAIN=.zagforge.com` already in `turbo.json` env list. Drop `ThemeProvider` + `themeSyncScript` from `@workspace/ui` into `apps/cloud/app/layout.tsx`. Dark/light preference syncs across zagforge.com, docs.zagforge.com, and cloud.zagforge.com with no extra work.

---

## 8. Phase Boundary (what's explicitly out of Phase 3)

| Item | Phase |
|------|-------|
| Analytics API + charts (LOC trends, language breakdown) | 4 |
| Stripe billing + plan enforcement | 4 |
| GitHub webhook automation (auto-snapshot on push) | 3b |
| GitLab / Bitbucket providers | 5+ |
| Custom system prompt UI | 4 |
| SOC2, PII scrubbing, audit logs | 5+ |
| Vector database / semantic search (RAG) | 5+ |
| Multi-model toggle in Query Console | 4 |
| Token usage tracking | 4 |

---

## 9. Success Criteria

Phase 3 is done when a developer can:

1. Run `zigzag --upload` and see their repo appear in cloud.zagforge.com
2. Copy a `zf_ctx_...` URL and paste it into Cursor Rules or a Claude Project
3. Ask a question in the Query Console and get a code-aware streamed answer
4. Invite a teammate to their org via Clerk's built-in invite flow
5. Add and remove AI provider keys from Settings
