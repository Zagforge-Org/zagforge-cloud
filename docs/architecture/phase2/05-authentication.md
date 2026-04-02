# Zagforge — Authentication & Security [Phase 2]

## Identity Provider: Zitadel

Zagforge uses a self-hosted [Zitadel](https://zitadel.com) instance as the identity provider, deployed as a standalone Cloud Run container (`auth.zagforge.com`). Zitadel handles:

- User registration (email/password with verification)
- SSO login (Google, GitHub via OIDC)
- Password reset flow
- Email verification
- JWT issuance (OIDC-compliant)
- Session lifecycle
- MFA (future)

The Go API does **not** implement any authentication logic. It validates Zitadel-issued JWTs via JWKS (local verification, no per-request call to Zitadel) and manages application-level concerns: user/org sync, org CRUD, session display, audit logging.

### JWT Verification

The `Auth()` middleware fetches Zitadel's JWKS from `{ZITADEL_ISSUER_URL}/.well-known/jwks.json` at startup (with periodic refresh) and validates JWTs locally:

1. Extract `Authorization: Bearer <token>` from request header
2. Verify signature against cached JWKS
3. Validate `iss`, `aud` (project ID), and `exp` claims
4. Extract `sub` (Zitadel user ID) and org claims
5. Store parsed claims in request context

### Scope Resolution

Every authenticated request targets either a **personal workspace** or an **organization**:

- Personal: derived from the JWT `sub` claim → `user_id`
- Organization: derived from a path parameter or header (e.g. `X-Org-ID`) → looked up via `zitadel_org_id` → `org_id`

The `Scope()` middleware (replaces the old `OrgScope()`) resolves this and stores the active `user_id` and optional `org_id` in the request context. All downstream queries filter by the resolved scope.

### Zitadel Webhooks

Zitadel sends event webhooks to `POST /internal/webhooks/zitadel` for:

- `user.created` → upsert into `users` table
- `user.updated` → update `users` table (username, email, avatar changes)
- `user.deleted` → cascade delete user and owned resources
- `org.created` / `org.updated` → upsert into `organizations` table
- `org.member.added` / `org.member.removed` → update `memberships` table
- `session.created` / `session.terminated` → update `sessions` table

Webhook requests are verified via Zitadel's signing key (configured in Zitadel Action settings).

### Account Management Endpoints

These live in the Go API and proxy to Zitadel's Management API where needed:

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/v1/account` | Get current user profile (from local DB) |
| `PATCH` | `/api/v1/account` | Update username, email, phone → calls Zitadel Management API + updates local DB |
| `DELETE` | `/api/v1/account` | Delete account → calls Zitadel Management API + cascade delete in local DB |
| `GET` | `/api/v1/account/sessions` | List active sessions (from local DB) |
| `DELETE` | `/api/v1/account/sessions/{id}` | Revoke session → calls Zitadel API + deletes from local DB |

### Organization Endpoints

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/v1/orgs` | Create organization → creates in Zitadel + local DB |
| `GET` | `/api/v1/orgs` | List user's organizations (from memberships table) |
| `PATCH` | `/api/v1/orgs/{orgID}` | Update org name/slug (owner/admin only) |
| `DELETE` | `/api/v1/orgs/{orgID}` | Delete organization (owner only) |
| `GET` | `/api/v1/orgs/{orgID}/members` | List org members |
| `POST` | `/api/v1/orgs/{orgID}/members` | Invite member (sends invite via email) |
| `PATCH` | `/api/v1/orgs/{orgID}/members/{userID}` | Change member role |
| `DELETE` | `/api/v1/orgs/{orgID}/members/{userID}` | Remove member |
| `GET` | `/api/v1/orgs/{orgID}/audit-log` | View org audit log (owner/admin only) |

---

## Internal Endpoint Authentication

All `/internal/*` endpoints are protected. Each uses the appropriate auth mechanism:

| Endpoint | Auth mechanism |
|---|---|
| `POST /internal/webhooks/github` | GitHub App webhook secret (HMAC-SHA256 via `X-Hub-Signature-256` header) |
| `POST /internal/webhooks/zitadel` | Zitadel webhook signing key |
| `POST /internal/jobs/start` | Signed job token (HMAC, same as complete) |
| `POST /internal/jobs/complete` | Signed job token (HMAC) |
| `POST /internal/watchdog/timeout` | GCP OIDC token (Cloud Scheduler service account) |

---

## Signed Job Tokens (Callback Security)

When the API creates a job and pushes it to Cloud Tasks, it generates a short-lived HMAC token:

```
token = HMAC-SHA256(service_secret, job_id + ":" + expiry_timestamp)
```

The `service_secret` is stored in Google Secret Manager. The token is passed to the Cloud Run Job as an environment variable. The worker includes it in both the `start` and `complete` callbacks:

```
POST /internal/jobs/complete
Authorization: Bearer <signed_token>

{
  "job_id": "abc123",
  "status": "succeeded",
  "snapshot_path": "gs://zagforge-snapshots/<org_uuid>/<repo_uuid>/3fa912e1/snapshot.json",
  "zigzag_version": "0.11.0",
  "size_bytes": 58320,
  "duration_ms": 4821
}
```

**Validation:** The API extracts `job_id` from the request body, re-derives `HMAC-SHA256(service_secret, job_id + ":" + expiry)`, and compares to the submitted token using constant-time comparison. The token is only valid for the specific `job_id` it was issued for.

**Start callback body:**

```json
{
  "job_id": "abc123"
}
```

The start endpoint transitions the job from `queued` → `running` and sets `started_at`.

---

## Callback Idempotency

The callback endpoint is idempotent. The job update and snapshot insert happen in a single transaction:

```
BEGIN;

SELECT status FROM jobs WHERE id = $job_id FOR UPDATE;

if status IN ('succeeded', 'failed'):
    COMMIT;
    return 200 OK (no-op)

UPDATE jobs SET status = $status, ...;
INSERT INTO snapshots (...) ON CONFLICT (repo_id, branch, commit_sha) DO NOTHING;

COMMIT;
```

The `SELECT ... FOR UPDATE` prevents concurrent callbacks from racing. The `ON CONFLICT DO NOTHING` on the unique snapshots index prevents duplicate snapshot rows.

---

## CORS

The public API serves JSON to any origin (AI tools, CLIs, custom integrations):

```
Access-Control-Allow-Origin: *
Access-Control-Allow-Methods: GET, OPTIONS
Access-Control-Allow-Headers: Authorization, Content-Type
```

No `POST`/`PUT`/`DELETE` on the public API in Phase 1 (read-only). Internal endpoints do not set CORS headers.

**Phase 5 addition:** `/v1/context/*` endpoints also carry `Access-Control-Allow-Origin: *`. Callers are AI tools (Cursor, Claude Projects) that are not browser-origin-constrained, but some tooling sends CORS preflight regardless.

---

## Request Validation

- All path parameters validated against expected formats (UUID, slug regex)
- Query parameters (`page`, `per_page`) clamped to safe ranges
- Request body size limited to 1MB on internal callback endpoints
- Content-Type enforcement on POST endpoints

---

## Secrets Rotation

All secrets stored in Google Secret Manager with versioning enabled:

| Secret | Rotation strategy |
|---|---|
| GitHub App private key | Manual, on compromise |
| GitHub App webhook secret | Manual, on compromise |
| HMAC signing key (job tokens) | Rotate quarterly; accept both current and previous version during transition |
| Redis auth password | Rotate with Memorystore instance recreation |

The HMAC signing key supports dual-version validation: when rotated, the API tries the current key first, then falls back to the previous version. This gives a grace period for in-flight jobs.

---

## Config Loading

Uses `caarlos0/env` for struct-based environment parsing with validation. Config is grouped by concern so handlers only receive the subset they need (e.g., pass `cfg.GitHub` to the webhook handler, not the entire config):

```go
type Config struct {
    Port    string `env:"PORT,required"`
    AppEnv  string `env:"APP_ENV,required"` // "dev" | "staging" | "prod"

    DB     DBConfig
    Redis  RedisConfig
    GCS    GCSConfig
    GitHub GitHubConfig
    Auth   AuthConfig

    ShutdownTimeout time.Duration `env:"SHUTDOWN_TIMEOUT" envDefault:"10s"`
}

type DBConfig struct {
    URL string `env:"DATABASE_URL,required"`
}

type RedisConfig struct {
    URL string `env:"REDIS_URL,required"`
}

type GCSConfig struct {
    Bucket   string `env:"GCS_BUCKET,required"`
    Endpoint string `env:"GCS_ENDPOINT"` // override for fake-gcs in dev
}

type GitHubConfig struct {
    AppID         int64  `env:"GITHUB_APP_ID,required"`
    PrivateKey    string `env:"GITHUB_APP_PRIVATE_KEY,required"`
    WebhookSecret string `env:"GITHUB_APP_WEBHOOK_SECRET,required"`
}

type AuthConfig struct {
    HMACSigningKey   string `env:"HMAC_SIGNING_KEY,required"`
    ZitadelIssuerURL string `env:"ZITADEL_ISSUER_URL,required"` // e.g. "https://auth.zagforge.com"
    ZitadelProjectID string `env:"ZITADEL_PROJECT_ID,required"`
}
```

`caarlos0/env` supports nested structs with `envPrefix` — each group can be passed independently to the component that needs it, avoiding the pattern of passing full config to every handler.

---

---

## Phase 5 Additions

### CLI Token Authentication

`POST /api/v1/upload` uses a separate auth mechanism from Zitadel OIDC session JWTs. CLI tokens are long-lived API keys issued by Zagforge, formatted as `zf_pk_<random>`.

The Go API uses a distinct middleware for this endpoint — it does **not** go through OIDC JWT verification. Implementation uses a DB-backed `cli_api_keys` table (token stored hashed, scoped to a user or org).

The middleware resolves the token to a `user_id` or `org_id` and enforces that the upload targets that scope only.

### Context Token Authentication

`GET /v1/context/{raw_token}` and `HEAD /v1/context/{raw_token}` use no `Authorization` header. The raw token in the URL path is the credential. The middleware:

1. Computes `SHA-256(raw_token)`
2. Looks up `context_tokens` by `token_hash`
3. Returns 404 if not found, 410 Gone if `expires_at < now()`
4. Passes resolved `context_token` row to the handler

Rate limiting uses Redis key `rl:ctx:{token_hash}` (60 req/min), separate namespace from `/api/v1/` rate limiter keys.

### AI Key Encryption

AI provider keys are stored AES-256-GCM encrypted in `ai_provider_keys.key_cipher`.

- `key_cipher` format: `nonce (12 bytes) || ciphertext`
- Encryption key: fetched from Google Secret Manager at startup, never in env or source
- Decryption: `key_cipher[:12]` is the nonce; `key_cipher[12:]` is the ciphertext
- `key_hint`: last 4 chars of the raw key, stored plaintext for UI display only

Provider key selection order for Query Console (Phase 5, hardcoded): Anthropic → OpenAI → Google.

---

## Graceful Server Shutdown

Follows the `shared/server` pattern — `signal.NotifyContext` + channel-based shutdown:

```go
func main() {
    logger.InitLogger(
        logger.GetEnv("APP_LOG_LEVEL", "info"),
        logger.GetEnv("APP_LOG_FOLDER", ""),
        logger.GetEnv("APP_ENV", "dev"),
        map[string]any{"pid": os.Getpid(), "service": "api"},
    )
    defer logger.Sync()

    cfg, err := config.New().Load()
    if err != nil {
        logger.Logger.Fatalf("failed to load config: %v", err)
    }

    // ... setup router, middleware, handlers ...

    server := webserver.NewServer(":" + cfg.Port)
    // register middleware and routes...

    go func() {
        if err := server.Start(); err != nil && err != http.ErrServerClosed {
            logger.Logger.Fatalf("server exited: %v", err)
        }
    }()

    <-server.Shutdown()
}
```

Cloud Run sends `SIGTERM` and waits up to 10 seconds by default. The Cloud Run service config should set `--timeout=60` to allow the drain window.
