# Zagforge — API Endpoints [Phase 2]

## Public API (Zitadel OIDC JWT auth)

`{org}` is the organization `slug`. `{repo}` is the repo `full_name` suffix (e.g., for "LegationPro/zigzag", the repo param is "zigzag").

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/v1/{org}/{repo}/latest` | Latest snapshot for default branch |
| `GET` | `/api/v1/{org}/{repo}/branches/{branch}/latest` | Latest snapshot for a specific branch |
| `GET` | `/api/v1/{org}/{repo}/snapshots` | List snapshot history |
| `GET` | `/api/v1/{org}/{repo}/snapshots/{id}` | Specific snapshot by ID |
| `GET` | `/api/v1/{org}/{repo}/jobs` | List jobs for a repo |

## Internal API

| Method | Path | Auth | Description |
|---|---|---|---|
| `POST` | `/internal/webhooks/github` | GitHub App HMAC | Webhook receiver |
| `POST` | `/internal/jobs/start` | Signed job token | Worker reports job started |
| `POST` | `/internal/jobs/complete` | Signed job token | Worker reports job finished |
| `POST` | `/internal/watchdog/timeout` | GCP OIDC token | Cloud Scheduler timeout check |

## GitHub App

| Method | Path | Description |
|---|---|---|
| `GET` | `/auth/github/install` | Redirect to GitHub App installation |
| `GET` | `/auth/github/callback` | Handle installation callback |

---

## Phase 5 Additions

### CLI Upload

```
POST /api/v1/upload
Auth: Zagforge CLI token (Authorization: Bearer zf_pk_<token>)
      Verified by CLI token middleware — distinct from the Zitadel OIDC JWT session middleware.
Body: {
  org_slug: string,           // Organization slug
  repo_full_name: string,     // "github-owner/repo-name"
  commit_sha: string,
  branch: string,
  metadata_snapshot: object   // snapshot_version 2 JSON (file tree + SHAs, no content)
}

Backend logic:
  1. Verify CLI bearer token via token middleware
  2. Resolve org_slug → org_id; verify token is scoped to that org
  3. Look up repositories by repo_full_name + org_id
     → Not found, no GitHub App installation: 404 { "error": "repo_not_connected" }
     → Not found but installation_id known: auto-create repositories row
  4. Insert snapshot row (job_id = NULL)
  5. Write snapshot.json to GCS

Response: { snapshot_id: uuid, created_at: timestamp }
```

### Context URL

```
HEAD /v1/context/{raw_token}
  Auth: none (token is the credential)
  → SHA-256(raw_token) lookup
  → Not found: 404
  → Expired: 410 Gone
  → Found: 200 OK + headers { X-Snapshot-ID, X-Commit-SHA, Content-Type: text/markdown }
  → No GitHub fetch triggered (lightweight ping/validation)

GET /v1/context/{raw_token}?format=markdown|json
  Auth: none
  CORS: Access-Control-Allow-Origin: * (callers are AI tools, not browser origins)
  Rate limit: 60 req/min per token_hash (Redis key: rl:ctx:{token_hash})
  → SHA-256(raw_token) lookup
  → Not found: 404  |  Expired: 410 Gone
  → Update last_used_at async (fire-and-forget goroutine)
  → Resolve snapshot (target_snapshot_id or latest)
  → v1 snapshot (no file_tree): 422 { "error": "snapshot_outdated" }
  → Check Redis LRU (key: ctx:{repo_id}:{commit_sha}, TTL 10 min)
  → Cache hit: stream immediately
  → Cache miss — streaming assembly pipeline:
      1. Flush markdown header immediately
      2. Per file in file_tree: fetch from GitHub via installation token, wrap in code block, flush
      3. Store assembled result in Redis
  → Response: text/markdown (or application/json)
```

### Query Console

```
POST /api/v1/{org}/{repo}/query
Auth: Zitadel OIDC JWT
Body: { question: string, snapshot_id?: uuid }
Response: text/event-stream (SSE)

{org} = organization slug, {repo} = repository full_name suffix (e.g. "zigzag")

Backend logic:
  1. Load snapshot (snapshot_id or latest)
  2. v1 snapshot: 422
  3. Assemble context (Redis LRU or streaming GitHub fetch, same pipeline as GET /v1/context/)
  4. Select AI provider: first key available in order Anthropic → OpenAI → Google
     No key configured: 422 { "error": "no_ai_key" }
  5. Decrypt key (AES-256-GCM, nonce prepended, encryption key from Secret Manager)
  6. Prepend hardcoded system prompt (Phase 5; user-editable system prompts are future)
  7. Stream provider response via SSE
```

### Context Token Management

| Method | Path | Auth | Description |
|---|---|---|---|
| `GET` | `/api/v1/{org}/{repo}/context-tokens` | Zitadel OIDC JWT | List tokens (label, key_hint, last_used_at, expires_at) |
| `POST` | `/api/v1/{org}/{repo}/context-tokens` | Zitadel OIDC JWT | Create token — raw token returned once only |
| `DELETE` | `/api/v1/{org}/{repo}/context-tokens/{id}` | Zitadel OIDC JWT | Revoke token |

### AI Key Management

| Method | Path | Auth | Description |
|---|---|---|---|
| `PUT` | `/api/v1/{org}/settings/ai-keys` | Zitadel OIDC JWT | Store encrypted key `{ provider, raw_key }` |
| `DELETE` | `/api/v1/{org}/settings/ai-keys/{provider}` | Zitadel OIDC JWT | Remove key |
| `GET` | `/api/v1/{org}/settings/ai-keys` | Zitadel OIDC JWT | List `{ provider, key_hint, created_at }` — never raw key |
