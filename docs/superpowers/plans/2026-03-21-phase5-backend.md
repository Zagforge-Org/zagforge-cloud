# Phase 5 Backend Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the Context Proxy backend to the Go API — CLI upload endpoint, context URL, query console, context token CRUD, AI key management, and all supporting services (encryption, assembly, cache).

**Architecture:** New tables and sqlc queries live in `shared/go/store/` (sqlc generates there from migration schema + query files). New middleware, handlers, and services live in `api/internal/`. Everything is wired into `api/cmd/main.go`. No existing handlers are modified.

**Tech Stack:** Go, Chi (via `shared/go/router`), pgx v5, sqlc v1.30, Redis (`go-redis/v9`), `shared/go/storage` (GCS), AES-256-GCM, SSE (`text/event-stream`).

**Spec:** `docs/superpowers/specs/2026-03-21-phase3-design.md`
**Architecture docs:** `architecture/phase5/15-context-proxy.md`, `architecture/phase5/16-dashboard.md`, `architecture/phase5/17-cli-upload.md`

**Key API facts (verified from source):**
- `httputil.WriteJSON[T](w, status, v)` — write any status + JSON body
- `httputil.ErrResponse(w, status, err)` — write error JSON
- `httputil.OkResponse[T](w, data)` — write 200 + JSON body
- `httputil.ParseUUID(r, "param")` — extract chi URL param as `pgtype.UUID`
- `storage.Client` — type name (not `GCSClient`); methods: `Upload(ctx, path, data)` and `Download(ctx, path) ([]byte, error)`
- `router.Method` constants: GET, POST, PUT, DELETE, PATCH — **no HEAD**; HEAD route needs chi directly (see Task 9)

---

## File Map

**New migration:**
- Create: `api/internal/db/migrations/000002_phase5.up.sql`
- Create: `api/internal/db/migrations/000002_phase5.down.sql`

**New SQL queries (sqlc reads from these files):**
- Modify: `shared/go/store/queries/snapshots.sql` — add `InsertCLISnapshot`
- Modify: `shared/go/store/queries/organizations.sql` — add `GetOrganizationBySlug`
- Modify: `shared/go/store/queries/repositories.sql` — add `GetRepoByFullNameAndOrg`
- Create: `shared/go/store/queries/ai_provider_keys.sql`
- Create: `shared/go/store/queries/context_tokens.sql`

**sqlc-generated (run `sqlc generate` from `api/` after migration + queries):**
- `shared/go/store/models.go` — updated Snapshot + new AiProviderKey, ContextToken types
- `shared/go/store/snapshots.sql.go` — updated
- `shared/go/store/ai_provider_keys.sql.go` — new
- `shared/go/store/context_tokens.sql.go` — new

**Router extension (add HEAD method):**
- Modify: `shared/go/router/group.go` — add `HEAD Method = "HEAD"` constant and handler

**New config fields:**
- Modify: `api/internal/config/app.go` — add `EncryptionKeyBase64`, `CLIAPIKey`

**Shared auth helper:**
- Create: `api/internal/middleware/auth/orgid.go` — `ResolveOrgID(ctx, claims, db)` helper

**New services:**
- Create: `api/internal/service/encryption/encryption.go`
- Create: `api/internal/service/encryption/encryption_test.go`
- Create: `api/internal/service/assembly/assembly.go`
- Create: `api/internal/service/assembly/assembly_test.go`

**New cache:**
- Create: `api/internal/cache/contextcache/contextcache.go`
- Create: `api/internal/cache/contextcache/contextcache_test.go`

**New middleware:**
- Create: `api/internal/middleware/clitoken/clitoken.go`
- Create: `api/internal/middleware/clitoken/clitoken_test.go`

**New handlers:**
- Create: `api/internal/handler/upload/upload.go` + `upload_test.go`
- Create: `api/internal/handler/contexturl/context.go` + `context_test.go`
- Create: `api/internal/handler/contexttokens/handler.go` + `handler_test.go`
- Create: `api/internal/handler/aikeys/handler.go` + `handler_test.go`
- Create: `api/internal/handler/query/query.go` + `query_test.go`

**Wire up:**
- Modify: `api/cmd/main.go`

---

## Task 1: Database Migration

**Files:**
- Create: `api/internal/db/migrations/000002_phase5.up.sql`
- Create: `api/internal/db/migrations/000002_phase5.down.sql`

- [ ] **Step 1: Write the up migration**

```sql
-- api/internal/db/migrations/000002_phase5.up.sql

-- CLI uploads have no associated job; make job_id nullable.
ALTER TABLE snapshots ALTER COLUMN job_id DROP NOT NULL;

-- Store ignore_patterns and zigzag_config_hash from the Zigzag run.
ALTER TABLE snapshots ADD COLUMN metadata JSONB;

-- Encrypted AI provider API keys, one per provider per org.
CREATE TABLE ai_provider_keys (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id      UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    provider    TEXT NOT NULL CHECK (provider IN ('openai', 'anthropic', 'google')),
    key_cipher  BYTEA NOT NULL,   -- nonce (12 bytes) || AES-256-GCM ciphertext
    key_hint    TEXT NOT NULL,    -- last 4 chars for display e.g. "...xK9f"
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (org_id, provider)
);

-- Revocable context URL tokens scoped to a repository.
CREATE TABLE context_tokens (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    repo_id            UUID NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    org_id             UUID NOT NULL REFERENCES organizations(id),
    target_snapshot_id UUID REFERENCES snapshots(id) ON DELETE SET NULL,
    token_hash         TEXT UNIQUE NOT NULL,
    label              TEXT,
    last_used_at       TIMESTAMPTZ,
    expires_at         TIMESTAMPTZ,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_context_tokens_hash ON context_tokens (token_hash);
```

- [ ] **Step 2: Write the down migration**

```sql
-- api/internal/db/migrations/000002_phase5.down.sql
DROP TABLE IF EXISTS context_tokens;
DROP TABLE IF EXISTS ai_provider_keys;
ALTER TABLE snapshots DROP COLUMN IF EXISTS metadata;
ALTER TABLE snapshots ALTER COLUMN job_id SET NOT NULL;
```

- [ ] **Step 3: Apply the migration locally**

```bash
task migrate:up
```

Expected: migration applies with no errors. Verify with `psql` or `task db:shell`:
```sql
\d snapshots   -- job_id should now be nullable, metadata column present
\dt            -- ai_provider_keys and context_tokens should appear
```

- [ ] **Step 4: Commit**

```bash
git add api/internal/db/migrations/
git commit -m "feat(db): phase 5 migration — nullable job_id, ai_provider_keys, context_tokens"
```

---

## Task 2: sqlc Queries

> **Important:** sqlc reads schema from migration files. Task 1 Step 3 (apply migration) must complete **before** running `sqlc generate` in Step 5 — sqlc also connects to validate, and the new columns/tables must exist.

**Files:**
- Modify: `shared/go/store/queries/snapshots.sql`
- Modify: `shared/go/store/queries/organizations.sql`
- Modify: `shared/go/store/queries/repositories.sql`
- Create: `shared/go/store/queries/ai_provider_keys.sql`
- Create: `shared/go/store/queries/context_tokens.sql`

- [ ] **Step 1: Add CLI snapshot insert to snapshots.sql**

Append to `shared/go/store/queries/snapshots.sql`:

```sql
-- name: InsertCLISnapshot :one
INSERT INTO snapshots (repo_id, branch, commit_sha, gcs_path, snapshot_version, zigzag_version, size_bytes, metadata)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (repo_id, branch, commit_sha) DO UPDATE
  SET gcs_path         = EXCLUDED.gcs_path,
      snapshot_version = EXCLUDED.snapshot_version,
      zigzag_version   = EXCLUDED.zigzag_version,
      size_bytes       = EXCLUDED.size_bytes,
      metadata         = EXCLUDED.metadata
RETURNING id, repo_id, job_id, branch, commit_sha, gcs_path, snapshot_version, zigzag_version, size_bytes, metadata, created_at;
```

- [ ] **Step 2: Add missing org query to organizations.sql**

Append to `shared/go/store/queries/organizations.sql`:

```sql
-- name: GetOrganizationBySlug :one
SELECT * FROM organizations WHERE slug = $1;
```

- [ ] **Step 3: Add missing repo query to repositories.sql**

Append to `shared/go/store/queries/repositories.sql`:

```sql
-- name: GetRepoByFullNameAndOrg :one
SELECT * FROM repositories WHERE full_name = $1 AND org_id = $2;
```

- [ ] **Step 4: Write ai_provider_keys queries**

```sql
-- shared/go/store/queries/ai_provider_keys.sql

-- name: UpsertAIProviderKey :one
INSERT INTO ai_provider_keys (org_id, provider, key_cipher, key_hint)
VALUES ($1, $2, $3, $4)
ON CONFLICT (org_id, provider) DO UPDATE
  SET key_cipher = EXCLUDED.key_cipher,
      key_hint   = EXCLUDED.key_hint
RETURNING id, org_id, provider, key_cipher, key_hint, created_at;

-- name: GetAIProviderKey :one
SELECT id, org_id, provider, key_cipher, key_hint, created_at
FROM ai_provider_keys
WHERE org_id = $1 AND provider = $2;

-- name: ListAIProviderKeys :many
SELECT id, org_id, provider, key_hint, created_at
FROM ai_provider_keys
WHERE org_id = $1
ORDER BY provider ASC;

-- name: DeleteAIProviderKey :exec
DELETE FROM ai_provider_keys WHERE org_id = $1 AND provider = $2;
```

- [ ] **Step 5: Write context_tokens queries**

```sql
-- shared/go/store/queries/context_tokens.sql

-- name: InsertContextToken :one
INSERT INTO context_tokens (repo_id, org_id, target_snapshot_id, token_hash, label, expires_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, repo_id, org_id, target_snapshot_id, token_hash, label, last_used_at, expires_at, created_at;

-- name: GetContextTokenByHash :one
SELECT id, repo_id, org_id, target_snapshot_id, token_hash, label, last_used_at, expires_at, created_at
FROM context_tokens
WHERE token_hash = $1;

-- name: ListContextTokensByRepo :many
SELECT id, repo_id, org_id, target_snapshot_id, label, last_used_at, expires_at, created_at
FROM context_tokens
WHERE repo_id = $1
ORDER BY created_at DESC;

-- name: UpdateContextTokenLastUsed :exec
UPDATE context_tokens SET last_used_at = now() WHERE id = $1;

-- name: DeleteContextToken :exec
DELETE FROM context_tokens WHERE id = $1 AND org_id = $2;
```

- [ ] **Step 6: Regenerate sqlc**

```bash
cd api && sqlc generate
```

Expected: `shared/go/store/` updated. Check that `models.go` now contains `AiProviderKey` and `ContextToken` structs, and that `Snapshot.JobID` is still `pgtype.UUID` (pgx v5 represents nullable UUIDs via `Valid bool` on the struct, no type change needed).

- [ ] **Step 7: Verify compile**

```bash
cd api && go build ./...
```

Expected: no errors.

- [ ] **Step 8: Commit**

```bash
git add shared/go/store/queries/ shared/go/store/
git commit -m "feat(db): phase 5 sqlc queries — CLI snapshot, org/repo lookups, ai_provider_keys, context_tokens"
```

---

## Task 3: Router — Add HEAD Method

`shared/go/router/group.go` defines GET, POST, PUT, DELETE, PATCH but not HEAD. The context URL endpoint needs HEAD.

**Files:**
- Modify: `shared/go/router/group.go`

- [ ] **Step 1: Write a failing test**

Check `shared/go/router/group_test.go` for the test pattern, then add:

```go
func TestHEADRoute(t *testing.T) {
	r := New()
	g := r.Group()
	g.Create([]Subroute{
		{Method: HEAD, Path: "/test", Handler: func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}},
	})
	req := httptest.NewRequest(http.MethodHead, "/test", nil)
	w := httptest.NewRecorder()
	r.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("got %d, want 200", w.Code)
	}
}
```

- [ ] **Step 2: Run — verify it fails**

```bash
cd shared/go && go test ./router/... -v -run TestHEADRoute
```

Expected: compile error — `HEAD` undefined.

- [ ] **Step 3: Add HEAD to the router**

In `shared/go/router/group.go`, add the constant and handle the method in the route registration switch:

```go
// In the Method constants block:
HEAD Method = "HEAD"
```

Then in the `Create` method's switch (wherever GET, POST etc. are mapped to chi), add:
```go
case HEAD:
    g.mux.Head(sub.Path, sub.Handler)
```

- [ ] **Step 4: Run — verify tests pass**

```bash
cd shared/go && go test ./router/... -v
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add shared/go/router/
git commit -m "feat(router): add HEAD method support"
```

---

## Task 4: Config — Encryption Key and CLI Key

**Files:**
- Modify: `api/internal/config/app.go`

- [ ] **Step 1: Add fields**

Read `api/internal/config/app.go`, then append to the `AppConfig` struct:

```go
// EncryptionKeyBase64 is a base64-encoded 32-byte AES-256 key for encrypting AI provider keys.
// Generate: openssl rand -base64 32
EncryptionKeyBase64 string `env:"ENCRYPTION_KEY_BASE64,required"`

// CLIAPIKey is the bearer token accepted on POST /api/v1/upload.
CLIAPIKey string `env:"CLI_API_KEY,required"`
```

- [ ] **Step 2: Set in dev env**

In `.env.dev` (or Doppler dev config):
```
ENCRYPTION_KEY_BASE64=<output of: openssl rand -base64 32>
CLI_API_KEY=zf_pk_devtestkey1234567890
```

- [ ] **Step 3: Verify compile**

```bash
cd api && go build ./...
```

- [ ] **Step 4: Commit**

```bash
git add api/internal/config/app.go
git commit -m "feat(config): add ENCRYPTION_KEY_BASE64 and CLI_API_KEY"
```

---

## Task 5: Shared Auth Helper — ResolveOrgID

Both `contexttokens` and `aikeys` handlers need to resolve the Clerk org from claims. Extract this as a shared helper.

**Files:**
- Create: `api/internal/middleware/auth/orgid.go`

- [ ] **Step 1: Write the failing test**

```go
// api/internal/middleware/auth/orgid_test.go
package auth_test

import (
	"testing"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/LegationPro/zagforge/api/internal/middleware/auth"
)

func TestResolveOrgIDMissingOrg(t *testing.T) {
	claims := &clerk.SessionClaims{} // no active org
	_, err := auth.ResolveClerkOrgID(claims)
	if err == nil {
		t.Fatal("expected error when no active org in claims")
	}
}
```

- [ ] **Step 2: Run — verify it fails**

```bash
cd api && go test ./internal/middleware/auth/... -v -run TestResolveOrgIDMissingOrg
```

- [ ] **Step 3: Implement**

```go
// api/internal/middleware/auth/orgid.go
package auth

import (
	"errors"

	"github.com/clerk/clerk-sdk-go/v2"
)

var ErrNoActiveOrg = errors.New("no active organization in session claims")

// ResolveClerkOrgID extracts the active Clerk organization ID from session claims.
// Returns ErrNoActiveOrg if the user has no org context in the current session.
func ResolveClerkOrgID(claims *clerk.SessionClaims) (string, error) {
	if claims.ActiveOrganizationID == "" {
		return "", ErrNoActiveOrg
	}
	return claims.ActiveOrganizationID, nil
}
```

> **Note:** Check what field `clerk.SessionClaims` actually uses for the org ID. Look at how the existing Clerk middleware in `auth.go` uses claims — the field may be `ActiveOrganizationID` or similar. If the Clerk SDK structures differ, adjust accordingly.

- [ ] **Step 4: Run — verify tests pass**

```bash
cd api && go test ./internal/middleware/auth/... -v
```

- [ ] **Step 5: Commit**

```bash
git add api/internal/middleware/auth/orgid.go api/internal/middleware/auth/orgid_test.go
git commit -m "feat(auth): ResolveClerkOrgID helper for handler org resolution"
```

---

## Task 6: Encryption Service

**Files:**
- Create: `api/internal/service/encryption/encryption.go`
- Create: `api/internal/service/encryption/encryption_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// api/internal/service/encryption/encryption_test.go
package encryption_test

import (
	"testing"

	"github.com/LegationPro/zagforge/api/internal/service/encryption"
)

func TestEncryptDecryptRoundtrip(t *testing.T) {
	key := make([]byte, 32)
	for i := range key { key[i] = byte(i) }

	svc, err := encryption.New(key)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	plaintext := []byte("sk-ant-api01-supersecret")
	cipher, err := svc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	got, err := svc.Decrypt(cipher)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if string(got) != string(plaintext) {
		t.Errorf("got %q, want %q", got, plaintext)
	}
}

func TestDecryptTamperedCiphertext(t *testing.T) {
	key := make([]byte, 32)
	svc, _ := encryption.New(key)
	cipher, _ := svc.Encrypt([]byte("secret"))
	cipher[len(cipher)-1] ^= 0xFF
	if _, err := svc.Decrypt(cipher); err == nil {
		t.Fatal("expected error on tampered ciphertext")
	}
}

func TestNewRejectsShortKey(t *testing.T) {
	if _, err := encryption.New([]byte("tooshort")); err == nil {
		t.Fatal("expected error for short key")
	}
}
```

- [ ] **Step 2: Run — verify it fails**

```bash
cd api && go test ./internal/service/encryption/... -v
```

- [ ] **Step 3: Implement**

```go
// api/internal/service/encryption/encryption.go
package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
)

// Service encrypts/decrypts using AES-256-GCM.
// Ciphertext format: nonce (12 bytes) || ciphertext.
type Service struct{ aead cipher.AEAD }

// New creates an encryption Service. key must be exactly 32 bytes.
func New(key []byte) (*Service, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("encryption: key must be 32 bytes, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Service{aead: aead}, nil
}

// Encrypt returns nonce || ciphertext.
func (s *Service) Encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("encryption: nonce: %w", err)
	}
	return s.aead.Seal(nonce, nonce, plaintext, nil), nil
}

// Decrypt expects nonce || ciphertext as produced by Encrypt.
func (s *Service) Decrypt(data []byte) ([]byte, error) {
	ns := s.aead.NonceSize()
	if len(data) < ns {
		return nil, errors.New("encryption: ciphertext too short")
	}
	plain, err := s.aead.Open(nil, data[:ns], data[ns:], nil)
	if err != nil {
		return nil, fmt.Errorf("encryption: decrypt: %w", err)
	}
	return plain, nil
}
```

- [ ] **Step 4: Run — verify tests pass**

```bash
cd api && go test ./internal/service/encryption/... -v
```

Expected: 3 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add api/internal/service/encryption/
git commit -m "feat(encryption): AES-256-GCM service"
```

---

## Task 7: Context Cache

**Files:**
- Create: `api/internal/cache/contextcache/contextcache.go`
- Create: `api/internal/cache/contextcache/contextcache_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// api/internal/cache/contextcache/contextcache_test.go
package contextcache_test

import (
	"context"
	"testing"

	"github.com/LegationPro/zagforge/api/internal/cache/contextcache"
)

func TestSetAndGet(t *testing.T) {
	c := contextcache.NewInMemory()
	key := contextcache.Key("repo-uuid-1", "abc123sha")
	want := "# Codebase\n\nsome content"

	if err := c.Set(context.Background(), key, want); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, ok, err := c.Get(context.Background(), key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !ok {
		t.Fatal("expected hit, got miss")
	}
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestGetMissing(t *testing.T) {
	c := contextcache.NewInMemory()
	_, ok, err := c.Get(context.Background(), contextcache.Key("x", "y"))
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if ok {
		t.Fatal("expected miss, got hit")
	}
}
```

- [ ] **Step 2: Run — verify it fails**

```bash
cd api && go test ./internal/cache/contextcache/... -v
```

- [ ] **Step 3: Implement**

```go
// api/internal/cache/contextcache/contextcache.go
package contextcache

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const ttl = 10 * time.Minute

// Key returns the Redis key for an assembled context.
// Namespace "ctx:" is separate from rate limiter keys ("rl:").
func Key(repoID, commitSHA string) string {
	return fmt.Sprintf("ctx:%s:%s", repoID, commitSHA)
}

// Cache is the interface for getting/setting assembled context strings.
type Cache interface {
	Get(ctx context.Context, key string) (string, bool, error)
	Set(ctx context.Context, key, value string) error
}

// RedisCache is the production implementation.
type RedisCache struct{ rdb *redis.Client }

func NewRedis(rdb *redis.Client) *RedisCache { return &RedisCache{rdb: rdb} }

func (c *RedisCache) Get(ctx context.Context, key string) (string, bool, error) {
	val, err := c.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return val, true, nil
}

func (c *RedisCache) Set(ctx context.Context, key, value string) error {
	return c.rdb.Set(ctx, key, value, ttl).Err()
}

// InMemoryCache is thread-safe, for unit tests only.
type InMemoryCache struct {
	mu    sync.RWMutex
	store map[string]string
}

func NewInMemory() *InMemoryCache { return &InMemoryCache{store: map[string]string{}} }

func (c *InMemoryCache) Get(_ context.Context, key string) (string, bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.store[key]
	return v, ok, nil
}

func (c *InMemoryCache) Set(_ context.Context, key, value string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store[key] = value
	return nil
}
```

- [ ] **Step 4: Run — verify tests pass**

```bash
cd api && go test ./internal/cache/contextcache/... -v
```

- [ ] **Step 5: Commit**

```bash
git add api/internal/cache/contextcache/
git commit -m "feat(cache): context assembly cache (Redis + in-memory for tests)"
```

---

## Task 8: CLI Token Middleware

**Files:**
- Create: `api/internal/middleware/clitoken/clitoken.go`
- Create: `api/internal/middleware/clitoken/clitoken_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// api/internal/middleware/clitoken/clitoken_test.go
package clitoken_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LegationPro/zagforge/api/internal/middleware/clitoken"
)

const testKey = "zf_pk_testkey1234567890"

func okH() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
}

func TestValidToken(t *testing.T) {
	mw := clitoken.Auth(testKey)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/upload", nil)
	req.Header.Set("Authorization", "Bearer "+testKey)
	w := httptest.NewRecorder()
	mw(okH()).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("got %d, want 200", w.Code)
	}
}

func TestMissingToken(t *testing.T) {
	mw := clitoken.Auth(testKey)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/upload", nil)
	w := httptest.NewRecorder()
	mw(okH()).ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("got %d, want 401", w.Code)
	}
}

func TestWrongToken(t *testing.T) {
	mw := clitoken.Auth(testKey)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/upload", nil)
	req.Header.Set("Authorization", "Bearer zf_pk_wrong")
	w := httptest.NewRecorder()
	mw(okH()).ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("got %d, want 401", w.Code)
	}
}
```

- [ ] **Step 2: Run — verify it fails**

```bash
cd api && go test ./internal/middleware/clitoken/... -v
```

- [ ] **Step 3: Implement**

```go
// api/internal/middleware/clitoken/clitoken.go
package clitoken

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"

	"github.com/LegationPro/zagforge/shared/go/httputil"
)

var (
	errMissing = errors.New("missing CLI API key")
	errInvalid = errors.New("invalid CLI API key")
)

// Auth returns middleware that validates a static CLI bearer token.
// Uses constant-time comparison to prevent timing attacks.
func Auth(validKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, found := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			if !found || token == "" {
				httputil.ErrResponse(w, http.StatusUnauthorized, errMissing)
				return
			}
			if subtle.ConstantTimeCompare([]byte(token), []byte(validKey)) != 1 {
				httputil.ErrResponse(w, http.StatusUnauthorized, errInvalid)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
```

- [ ] **Step 4: Run — verify tests pass**

```bash
cd api && go test ./internal/middleware/clitoken/... -v
```

Expected: 3 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add api/internal/middleware/clitoken/
git commit -m "feat(middleware): CLI token auth for POST /api/v1/upload"
```

---

## Task 9: Context Assembly Service

**Files:**
- Create: `api/internal/service/assembly/assembly.go`
- Create: `api/internal/service/assembly/assembly_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// api/internal/service/assembly/assembly_test.go
package assembly_test

import (
	"context"
	"strings"
	"testing"

	"github.com/LegationPro/zagforge/api/internal/service/assembly"
)

func TestAssembleMarkdown(t *testing.T) {
	files := []assembly.FileEntry{
		{Path: "main.go", Language: "go", SHA: "abc"},
		{Path: "README.md", Language: "markdown", SHA: "def"},
	}
	fetcher := assembly.FetcherFunc(func(_ context.Context, sha string) (string, error) {
		if sha == "abc" {
			return "package main", nil
		}
		return "# My Project", nil
	})

	var buf strings.Builder
	if err := assembly.Assemble(context.Background(), "org/repo", "deadbeef1234", files, fetcher, &buf); err != nil {
		t.Fatalf("Assemble: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "# Codebase Snapshot") {
		t.Error("missing header")
	}
	if !strings.Contains(out, "package main") {
		t.Error("missing go file content")
	}
	if !strings.Contains(out, "```go") {
		t.Error("missing go code fence")
	}
	if !strings.Contains(out, "# My Project") {
		t.Error("missing README content")
	}
}
```

- [ ] **Step 2: Run — verify it fails**

```bash
cd api && go test ./internal/service/assembly/... -v
```

- [ ] **Step 3: Implement**

```go
// api/internal/service/assembly/assembly.go
package assembly

import (
	"context"
	"fmt"
	"io"
)

// FileEntry is one entry from the snapshot file_tree.
type FileEntry struct {
	Path     string `json:"path"`
	Language string `json:"language"`
	Lines    int    `json:"lines"`
	SHA      string `json:"sha"`
}

// Fetcher retrieves raw file content by its Git blob SHA.
type Fetcher interface {
	FetchBlob(ctx context.Context, sha string) (string, error)
}

// FetcherFunc adapts a plain function to Fetcher.
type FetcherFunc func(ctx context.Context, sha string) (string, error)

func (f FetcherFunc) FetchBlob(ctx context.Context, sha string) (string, error) { return f(ctx, sha) }

// Assemble writes a report.llm.md-style markdown document to w.
// Flushes the header immediately; file content is streamed as each blob arrives.
func Assemble(ctx context.Context, repoFullName, commitSHA string, files []FileEntry, fetcher Fetcher, w io.Writer) error {
	shortSHA := commitSHA
	if len(shortSHA) > 8 {
		shortSHA = shortSHA[:8]
	}
	if _, err := fmt.Fprintf(w, "# Codebase Snapshot — %s @ %s\n\n", repoFullName, shortSHA); err != nil {
		return err
	}
	flush(w)

	for _, f := range files {
		content, err := fetcher.FetchBlob(ctx, f.SHA)
		if err != nil {
			return fmt.Errorf("fetch %s: %w", f.Path, err)
		}
		lang := f.Language
		if lang == "" {
			lang = "text"
		}
		if _, err := fmt.Fprintf(w, "## %s\n\n```%s\n%s\n```\n\n", f.Path, lang, content); err != nil {
			return err
		}
		flush(w)
	}
	return nil
}

func flush(w io.Writer) {
	if fl, ok := w.(interface{ Flush() }); ok {
		fl.Flush()
	}
}
```

- [ ] **Step 4: Run — verify tests pass**

```bash
cd api && go test ./internal/service/assembly/... -v
```

- [ ] **Step 5: Commit**

```bash
git add api/internal/service/assembly/
git commit -m "feat(assembly): streaming markdown assembler for context proxy"
```

---

## Task 10: Upload Handler

**Files:**
- Create: `api/internal/handler/upload/upload.go`
- Create: `api/internal/handler/upload/upload_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// api/internal/handler/upload/upload_test.go
package upload_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LegationPro/zagforge/api/internal/handler/upload"
	"go.uber.org/zap"
)

func TestUploadBadBody(t *testing.T) {
	h := upload.NewHandler(nil, nil, zap.NewNop())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/upload", bytes.NewBufferString("{bad json}"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Upload(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", w.Code)
	}
}

func TestUploadMissingFields(t *testing.T) {
	h := upload.NewHandler(nil, nil, zap.NewNop())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/upload",
		bytes.NewBufferString(`{"org_slug":""}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Upload(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", w.Code)
	}
}

func TestUploadWrongSnapshotVersion(t *testing.T) {
	h := upload.NewHandler(nil, nil, zap.NewNop())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/upload",
		bytes.NewBufferString(`{"org_slug":"acme","repo_full_name":"acme/app","commit_sha":"abc","branch":"main","metadata_snapshot":{"snapshot_version":1}}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Upload(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", w.Code)
	}
}
```

- [ ] **Step 2: Run — verify it fails**

```bash
cd api && go test ./internal/handler/upload/... -v
```

- [ ] **Step 3: Implement**

```go
// api/internal/handler/upload/upload.go
package upload

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"go.uber.org/zap"

	dbpkg "github.com/LegationPro/zagforge/api/internal/db"
	"github.com/LegationPro/zagforge/shared/go/httputil"
	"github.com/LegationPro/zagforge/shared/go/storage"
	store "github.com/LegationPro/zagforge/shared/go/store"
	"github.com/jackc/pgx/v5/pgtype"
)

type snapshotMetadata struct {
	SnapshotVersion int    `json:"snapshot_version"`
	ZigzagVersion   string `json:"zigzag_version"`
	CommitSHA       string `json:"commit_sha"`
	Branch          string `json:"branch"`
	Summary         any    `json:"summary"`
	FileTree        []struct {
		Path     string `json:"path"`
		Language string `json:"language"`
		Lines    int    `json:"lines"`
		SHA      string `json:"sha"`
	} `json:"file_tree"`
}

type uploadRequest struct {
	OrgSlug          string           `json:"org_slug"`
	RepoFullName     string           `json:"repo_full_name"`
	CommitSHA        string           `json:"commit_sha"`
	Branch           string           `json:"branch"`
	MetadataSnapshot snapshotMetadata `json:"metadata_snapshot"`
}

// Handler handles CLI snapshot uploads.
type Handler struct {
	db      *dbpkg.DB
	storage *storage.Client
	log     *zap.Logger
}

func NewHandler(db *dbpkg.DB, gcs *storage.Client, log *zap.Logger) *Handler {
	return &Handler{db: db, storage: gcs, log: log}
}

func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	var req uploadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}
	if req.OrgSlug == "" || req.RepoFullName == "" || req.CommitSHA == "" || req.Branch == "" {
		httputil.ErrResponse(w, http.StatusBadRequest, errMissingFields)
		return
	}
	if req.MetadataSnapshot.SnapshotVersion != 2 {
		httputil.ErrResponse(w, http.StatusBadRequest, errSnapshotVersion)
		return
	}

	ctx := r.Context()

	org, err := h.db.Queries.GetOrganizationBySlug(ctx, req.OrgSlug)
	if err != nil {
		httputil.ErrResponse(w, http.StatusNotFound, errOrgNotFound)
		return
	}

	repo, err := h.db.Queries.GetRepoByFullNameAndOrg(ctx, store.GetRepoByFullNameAndOrgParams{
		FullName: req.RepoFullName,
		OrgID:    org.ID,
	})
	if err != nil {
		httputil.ErrResponse(w, http.StatusNotFound, errRepoNotConnected)
		return
	}

	metaJSON, err := json.Marshal(req.MetadataSnapshot)
	if err != nil {
		h.log.Error("marshal snapshot", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, errInternal)
		return
	}

	gcsPath := fmt.Sprintf("%s/%s/%s/snapshot.json",
		org.ID.String(), repo.ID.String(), req.CommitSHA)

	if err := h.storage.Upload(ctx, gcsPath, metaJSON); err != nil {
		h.log.Error("write snapshot to gcs", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, errInternal)
		return
	}

	snap, err := h.db.Queries.InsertCLISnapshot(ctx, store.InsertCLISnapshotParams{
		RepoID:          repo.ID,
		Branch:          req.Branch,
		CommitSha:       req.CommitSHA,
		GcsPath:         gcsPath,
		SnapshotVersion: 2,
		ZigzagVersion:   req.MetadataSnapshot.ZigzagVersion,
		SizeBytes:       int64(len(metaJSON)),
		Metadata:        metaJSON,
	})
	if err != nil {
		h.log.Error("insert snapshot", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, errInternal)
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, map[string]any{
		"snapshot_id": snap.ID.String(),
		"created_at":  snap.CreatedAt.Time,
	})
}

var (
	errMissingFields    = errors.New("org_slug, repo_full_name, commit_sha, and branch are required")
	errSnapshotVersion  = errors.New("metadata_snapshot.snapshot_version must be 2")
	errOrgNotFound      = errors.New("organization not found")
	errRepoNotConnected = errors.New("repository not connected; install the Zagforge GitHub App first")
	errInternal         = errors.New("internal error")
)
```

> **Note:** Check the exact field names that sqlc generates for `InsertCLISnapshotParams` after running `sqlc generate`. The `Metadata` field will be typed as `[]byte` or `pgtype.Text` depending on sqlc config for JSONB — adjust if needed.

- [ ] **Step 4: Run tests**

```bash
cd api && go test ./internal/handler/upload/... -v
```

Expected: 3 tests PASS.

- [ ] **Step 5: Compile check**

```bash
cd api && go build ./...
```

- [ ] **Step 6: Commit**

```bash
git add api/internal/handler/upload/
git commit -m "feat(handler): POST /api/v1/upload — CLI snapshot upload"
```

---

## Task 11: Context URL Handler (HEAD + GET)

> **Package name:** This handler uses package `contexturl` (not `context`) to avoid shadowing the stdlib `context` package.

**Files:**
- Create: `api/internal/handler/contexturl/context.go`
- Create: `api/internal/handler/contexturl/context_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// api/internal/handler/contexturl/context_test.go
package contexturl_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LegationPro/zagforge/api/internal/handler/contexturl"
	"go.uber.org/zap"
)

func TestHeadUnknownToken(t *testing.T) {
	h := contexturl.NewHandler(nil, nil, nil, nil, zap.NewNop())
	req := httptest.NewRequest(http.MethodHead, "/v1/context/zf_ctx_doesnotexist", nil)
	req.SetPathValue("token", "zf_ctx_doesnotexist")
	w := httptest.NewRecorder()
	h.Head(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("got %d, want 404", w.Code)
	}
}

func TestGetUnknownToken(t *testing.T) {
	h := contexturl.NewHandler(nil, nil, nil, nil, zap.NewNop())
	req := httptest.NewRequest(http.MethodGet, "/v1/context/zf_ctx_doesnotexist", nil)
	req.SetPathValue("token", "zf_ctx_doesnotexist")
	w := httptest.NewRecorder()
	h.Get(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("got %d, want 404", w.Code)
	}
}
```

- [ ] **Step 2: Run — verify it fails**

```bash
cd api && go test ./internal/handler/contexturl/... -v
```

- [ ] **Step 3: Implement**

```go
// api/internal/handler/contexturl/context.go
package contexturl

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"

	dbpkg "github.com/LegationPro/zagforge/api/internal/db"
	"github.com/LegationPro/zagforge/api/internal/cache/contextcache"
	"github.com/LegationPro/zagforge/api/internal/service/assembly"
	"github.com/LegationPro/zagforge/shared/go/httputil"
	githubprovider "github.com/LegationPro/zagforge/shared/go/provider/github"
	"github.com/LegationPro/zagforge/shared/go/storage"
	store "github.com/LegationPro/zagforge/shared/go/store"
)

type Handler struct {
	db      *dbpkg.DB
	cache   contextcache.Cache
	github  githubprovider.Worker
	storage *storage.Client
	log     *zap.Logger
}

func NewHandler(db *dbpkg.DB, cache contextcache.Cache, gh githubprovider.Worker, gcs *storage.Client, log *zap.Logger) *Handler {
	return &Handler{db: db, cache: cache, github: gh, storage: gcs, log: log}
}

func tokenHash(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

// Head handles HEAD /v1/context/{token} — lightweight token validation, no GitHub fetch.
func (h *Handler) Head(w http.ResponseWriter, r *http.Request) {
	raw := r.PathValue("token")
	tok, err := h.db.Queries.GetContextTokenByHash(r.Context(), tokenHash(raw))
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if tok.ExpiresAt.Valid && tok.ExpiresAt.Time.Before(time.Now()) {
		w.WriteHeader(http.StatusGone)
		return
	}
	w.Header().Set("X-Snapshot-ID", tok.TargetSnapshotID.UUID.String())
	w.Header().Set("Content-Type", "text/markdown")
	w.WriteHeader(http.StatusOK)
}

// Get handles GET /v1/context/{token} — assembles and streams report.llm.md.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	raw := r.PathValue("token")
	tok, err := h.db.Queries.GetContextTokenByHash(r.Context(), tokenHash(raw))
	if err != nil {
		httputil.ErrResponse(w, http.StatusNotFound, errNotFound)
		return
	}
	if tok.ExpiresAt.Valid && tok.ExpiresAt.Time.Before(time.Now()) {
		httputil.ErrResponse(w, http.StatusGone, errExpired)
		return
	}

	// Update last_used_at async — not on the critical path.
	// Use context.Background() so the goroutine isn't cancelled when the handler returns.
	go func() {
		_ = h.db.Queries.UpdateContextTokenLastUsed(context.Background(), tok.ID)
	}()

	// Resolve snapshot.
	var snap store.Snapshot
	if tok.TargetSnapshotID.Valid {
		snap, err = h.db.Queries.GetSnapshotByID(r.Context(), tok.TargetSnapshotID.UUID)
	} else {
		repo, rerr := h.db.Queries.GetRepoByID(r.Context(), tok.RepoID)
		if rerr != nil {
			httputil.ErrResponse(w, http.StatusInternalServerError, errInternal)
			return
		}
		snap, err = h.db.Queries.GetLatestSnapshot(r.Context(), store.GetLatestSnapshotParams{
			RepoID: tok.RepoID, Branch: repo.DefaultBranch,
		})
	}
	if err != nil {
		httputil.ErrResponse(w, http.StatusNotFound, errSnapshotNotFound)
		return
	}
	if snap.SnapshotVersion < 2 {
		httputil.ErrResponse(w, http.StatusUnprocessableEntity, errSnapshotOutdated)
		return
	}

	cacheKey := contextcache.Key(tok.RepoID.UUID.String(), snap.CommitSha)

	// Cache hit — stream immediately.
	if cached, ok, _ := h.cache.Get(r.Context(), cacheKey); ok {
		w.Header().Set("Content-Type", "text/markdown")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		_, _ = io.WriteString(w, cached)
		return
	}

	// Cache miss — load snapshot metadata from GCS, stream assembly.
	metaBytes, err := h.storage.Download(r.Context(), snap.GcsPath)
	if err != nil {
		h.log.Error("download snapshot from gcs", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, errInternal)
		return
	}

	var meta struct {
		FileTree []assembly.FileEntry `json:"file_tree"`
	}
	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		h.log.Error("unmarshal snapshot metadata", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, errInternal)
		return
	}

	repo, err := h.db.Queries.GetRepoByID(r.Context(), tok.RepoID)
	if err != nil {
		httputil.ErrResponse(w, http.StatusInternalServerError, errInternal)
		return
	}

	// GitHub blob fetcher.
	// NOTE: requires GetBlob(ctx, installationID, repoFullName, sha) on the GitHub provider.
	// If this method doesn't exist on githubprovider.Worker, add it in shared/go/provider/github/api.go.
	fetcher := assembly.FetcherFunc(func(ctx context.Context, sha string) (string, error) {
		return h.github.GetBlob(ctx, repo.InstallationID, repo.FullName, sha)
	})

	w.Header().Set("Content-Type", "text/markdown")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	var assembled strings.Builder
	mw := io.MultiWriter(w, &assembled)

	if err := assembly.Assemble(r.Context(), repo.FullName, snap.CommitSha, meta.FileTree, fetcher, mw); err != nil {
		h.log.Error("assembly failed", zap.Error(err))
		return // partial response already flushed
	}

	// Cache the assembled result for the next request.
	go func() {
		_ = h.cache.Set(context.Background(), cacheKey, assembled.String())
	}()
}

var (
	errNotFound         = errors.New("context token not found")
	errExpired          = errors.New("context token has expired")
	errSnapshotNotFound = errors.New("snapshot not found")
	errSnapshotOutdated = errors.New("snapshot_outdated: re-run zigzag --upload to generate a v2 snapshot")
	errInternal         = errors.New("internal error")
)
```

> **Add `GetBlob` to the GitHub provider:** Check `shared/go/provider/github/api.go`. If `GetBlob(ctx, installationID int64, repoFullName, sha string) (string, error)` doesn't exist, add it. Call `GET /repos/{owner}/{repo}/git/blobs/{sha}` with an installation token (use the same `generateInstallationToken` pattern already in the file). Add it to the `Worker` interface in `shared/go/provider/github/worker.go`.

- [ ] **Step 4: Run tests**

```bash
cd api && go test ./internal/handler/contexturl/... -v
```

Expected: 2 tests PASS (nil DB returns 404 because the lookup returns an error).

- [ ] **Step 5: Compile check**

```bash
cd api && go build ./...
```

- [ ] **Step 6: Commit**

```bash
git add api/internal/handler/contexturl/
git commit -m "feat(handler): GET+HEAD /v1/context/{token} — context URL"
```

---

## Task 12: Context Token CRUD Handler

**Files:**
- Create: `api/internal/handler/contexttokens/handler.go`
- Create: `api/internal/handler/contexttokens/handler_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// api/internal/handler/contexttokens/handler_test.go
package contexttokens_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LegationPro/zagforge/api/internal/handler/contexttokens"
	"go.uber.org/zap"
)

// List with nil DB panics on DB call — test that handler initialises without panic.
func TestHandlerInitialises(t *testing.T) {
	h := contexttokens.NewHandler(nil, zap.NewNop())
	if h == nil {
		t.Fatal("expected non-nil handler")
	}
}
```

- [ ] **Step 2: Run — verify it fails**

```bash
cd api && go test ./internal/handler/contexttokens/... -v
```

- [ ] **Step 3: Implement**

```go
// api/internal/handler/contexttokens/handler.go
package contexttokens

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"

	dbpkg "github.com/LegationPro/zagforge/api/internal/db"
	"github.com/LegationPro/zagforge/api/internal/middleware/auth"
	"github.com/LegationPro/zagforge/shared/go/httputil"
	store "github.com/LegationPro/zagforge/shared/go/store"
)

type Handler struct {
	db  *dbpkg.DB
	log *zap.Logger
}

func NewHandler(db *dbpkg.DB, log *zap.Logger) *Handler {
	return &Handler{db: db, log: log}
}

// List returns context tokens for a repo (repoID from chi URL param).
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	repoID, err := httputil.ParseUUID(r, "repoID")
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}
	tokens, err := h.db.Queries.ListContextTokensByRepo(r.Context(), repoID)
	if err != nil {
		h.log.Error("list context tokens", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, errInternal)
		return
	}
	httputil.OkResponse(w, tokens)
}

// Create generates a new context token and returns the raw token once only.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.ClaimsFromContext(r.Context())
	if err != nil {
		httputil.ErrResponse(w, http.StatusUnauthorized, err)
		return
	}
	repoID, err := httputil.ParseUUID(r, "repoID")
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}

	var body struct {
		Label     string  `json:"label"`
		ExpiresAt *string `json:"expires_at"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	raw := generateToken()
	hash := sha256Hash(raw)

	clerkOrgID, err := auth.ResolveClerkOrgID(claims)
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}
	org, err := h.db.Queries.GetOrgByClerkID(r.Context(), clerkOrgID)
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, errors.New("org not found"))
		return
	}

	var expiresAt pgtype.Timestamptz
	if body.ExpiresAt != nil {
		t, perr := time.Parse(time.RFC3339, *body.ExpiresAt)
		if perr != nil {
			httputil.ErrResponse(w, http.StatusBadRequest, fmt.Errorf("invalid expires_at: %w", perr))
			return
		}
		expiresAt = pgtype.Timestamptz{Time: t, Valid: true}
	}

	tok, err := h.db.Queries.InsertContextToken(r.Context(), store.InsertContextTokenParams{
		RepoID:    repoID,
		OrgID:     org.ID,
		TokenHash: hash,
		Label:     pgtype.Text{String: body.Label, Valid: body.Label != ""},
		ExpiresAt: expiresAt,
	})
	if err != nil {
		h.log.Error("insert context token", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, errInternal)
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, map[string]any{
		"id":        tok.ID.String(),
		"raw_token": raw, // returned once only
		"label":     tok.Label.String,
		"created_at": tok.CreatedAt.Time,
		"expires_at": tok.ExpiresAt,
	})
}

// Delete revokes a context token (must belong to caller's org).
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.ClaimsFromContext(r.Context())
	if err != nil {
		httputil.ErrResponse(w, http.StatusUnauthorized, err)
		return
	}
	tokenID, err := httputil.ParseUUID(r, "tokenID")
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}
	clerkOrgID, err := auth.ResolveClerkOrgID(claims)
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}
	org, err := h.db.Queries.GetOrgByClerkID(r.Context(), clerkOrgID)
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, errors.New("org not found"))
		return
	}
	if err := h.db.Queries.DeleteContextToken(r.Context(), store.DeleteContextTokenParams{
		ID: tokenID, OrgID: org.ID,
	}); err != nil {
		h.log.Error("delete context token", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, errInternal)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func generateToken() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return "zf_ctx_" + base64.RawURLEncoding.EncodeToString(b)
}

func sha256Hash(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

var errInternal = errors.New("internal error")
```

- [ ] **Step 4: Run tests and compile**

```bash
cd api && go test ./internal/handler/contexttokens/... -v && go build ./...
```

- [ ] **Step 5: Commit**

```bash
git add api/internal/handler/contexttokens/
git commit -m "feat(handler): context token CRUD (list/create/delete)"
```

---

## Task 13: AI Keys Handler

**Files:**
- Create: `api/internal/handler/aikeys/handler.go`
- Create: `api/internal/handler/aikeys/handler_test.go`

- [ ] **Step 1: Write the failing test**

```go
// api/internal/handler/aikeys/handler_test.go
package aikeys_test

import (
	"testing"

	"github.com/LegationPro/zagforge/api/internal/handler/aikeys"
	"github.com/LegationPro/zagforge/api/internal/service/encryption"
	"go.uber.org/zap"
)

func TestHandlerInitialises(t *testing.T) {
	key := make([]byte, 32)
	enc, _ := encryption.New(key)
	h := aikeys.NewHandler(nil, enc, zap.NewNop())
	if h == nil {
		t.Fatal("expected non-nil handler")
	}
}
```

- [ ] **Step 2: Run — verify it fails**

```bash
cd api && go test ./internal/handler/aikeys/... -v
```

- [ ] **Step 3: Implement**

```go
// api/internal/handler/aikeys/handler.go
package aikeys

import (
	"encoding/json"
	"errors"
	"net/http"

	"go.uber.org/zap"

	dbpkg "github.com/LegationPro/zagforge/api/internal/db"
	"github.com/LegationPro/zagforge/api/internal/middleware/auth"
	"github.com/LegationPro/zagforge/api/internal/service/encryption"
	"github.com/LegationPro/zagforge/shared/go/httputil"
	store "github.com/LegationPro/zagforge/shared/go/store"
)

type Handler struct {
	db  *dbpkg.DB
	enc *encryption.Service
	log *zap.Logger
}

func NewHandler(db *dbpkg.DB, enc *encryption.Service, log *zap.Logger) *Handler {
	return &Handler{db: db, enc: enc, log: log}
}

// List returns provider names and key hints — never the raw key.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	org, err := h.resolveOrg(r)
	if err != nil {
		httputil.ErrResponse(w, http.StatusUnauthorized, err)
		return
	}
	keys, err := h.db.Queries.ListAIProviderKeys(r.Context(), org.ID)
	if err != nil {
		h.log.Error("list ai keys", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, errInternal)
		return
	}
	httputil.OkResponse(w, keys)
}

// Upsert stores an encrypted AI provider key.
func (h *Handler) Upsert(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Provider string `json:"provider"`
		RawKey   string `json:"raw_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}
	if body.Provider == "" || body.RawKey == "" {
		httputil.ErrResponse(w, http.StatusBadRequest, errMissingFields)
		return
	}

	cipher, err := h.enc.Encrypt([]byte(body.RawKey))
	if err != nil {
		h.log.Error("encrypt ai key", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, errInternal)
		return
	}

	hint := body.RawKey
	if len(hint) > 4 {
		hint = "..." + hint[len(hint)-4:]
	}

	org, err := h.resolveOrg(r)
	if err != nil {
		httputil.ErrResponse(w, http.StatusUnauthorized, err)
		return
	}

	if _, err := h.db.Queries.UpsertAIProviderKey(r.Context(), store.UpsertAIProviderKeyParams{
		OrgID:     org.ID,
		Provider:  body.Provider,
		KeyCipher: cipher,
		KeyHint:   hint,
	}); err != nil {
		h.log.Error("upsert ai key", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, errInternal)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Delete removes an AI provider key.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	provider := r.PathValue("provider")
	org, err := h.resolveOrg(r)
	if err != nil {
		httputil.ErrResponse(w, http.StatusUnauthorized, err)
		return
	}
	if err := h.db.Queries.DeleteAIProviderKey(r.Context(), store.DeleteAIProviderKeyParams{
		OrgID: org.ID, Provider: provider,
	}); err != nil {
		h.log.Error("delete ai key", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, errInternal)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) resolveOrg(r *http.Request) (store.Organization, error) {
	claims, err := auth.ClaimsFromContext(r.Context())
	if err != nil {
		return store.Organization{}, err
	}
	clerkOrgID, err := auth.ResolveClerkOrgID(claims)
	if err != nil {
		return store.Organization{}, err
	}
	return h.db.Queries.GetOrgByClerkID(r.Context(), clerkOrgID)
}

var (
	errMissingFields = errors.New("provider and raw_key are required")
	errInternal      = errors.New("internal error")
)
```

- [ ] **Step 4: Run tests and compile**

```bash
cd api && go test ./internal/handler/aikeys/... -v && go build ./...
```

- [ ] **Step 5: Commit**

```bash
git add api/internal/handler/aikeys/
git commit -m "feat(handler): AI provider key management (list/upsert/delete)"
```

---

## Task 14: Query Console Handler (SSE)

**Files:**
- Create: `api/internal/handler/query/query.go`
- Create: `api/internal/handler/query/query_test.go`

**Route note:** The query route uses `{repoID}` (UUID) rather than `{org}/{repo}` slugs. This keeps it consistent with existing API routes (`/api/v1/repos/{repoID}/...`). The frontend resolves the repo UUID from the repo list before calling this endpoint.

- [ ] **Step 1: Write the failing test**

```go
// api/internal/handler/query/query_test.go
package query_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/LegationPro/zagforge/api/internal/handler/query"
	"go.uber.org/zap"
)

func TestQueryBadBody(t *testing.T) {
	h := query.NewHandler(nil, nil, nil, nil, zap.NewNop())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/repos/x/query",
		strings.NewReader(`{bad json}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Query(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", w.Code)
	}
}

func TestQueryMissingQuestion(t *testing.T) {
	h := query.NewHandler(nil, nil, nil, nil, zap.NewNop())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/repos/x/query",
		strings.NewReader(`{"question":""}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Query(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", w.Code)
	}
}
```

- [ ] **Step 2: Run — verify it fails**

```bash
cd api && go test ./internal/handler/query/... -v
```

- [ ] **Step 3: Implement**

```go
// api/internal/handler/query/query.go
package query

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"go.uber.org/zap"

	dbpkg "github.com/LegationPro/zagforge/api/internal/db"
	"github.com/LegationPro/zagforge/api/internal/cache/contextcache"
	"github.com/LegationPro/zagforge/api/internal/middleware/auth"
	"github.com/LegationPro/zagforge/api/internal/service/assembly"
	"github.com/LegationPro/zagforge/api/internal/service/encryption"
	"github.com/LegationPro/zagforge/shared/go/httputil"
	githubprovider "github.com/LegationPro/zagforge/shared/go/provider/github"
	store "github.com/LegationPro/zagforge/shared/go/store"
)

const systemPrompt = `You are an expert on this codebase. Refer to files by their full path. Answer concisely and precisely. If unsure, say so.`

// providerOrder defines fallback preference for AI provider selection.
var providerOrder = []string{"anthropic", "openai", "google"}

type Handler struct {
	db     *dbpkg.DB
	cache  contextcache.Cache
	github githubprovider.Worker
	enc    *encryption.Service
	log    *zap.Logger
}

func NewHandler(db *dbpkg.DB, cache contextcache.Cache, gh githubprovider.Worker, enc *encryption.Service, log *zap.Logger) *Handler {
	return &Handler{db: db, cache: cache, github: gh, enc: enc, log: log}
}

type queryRequest struct {
	Question   string  `json:"question"`
	SnapshotID *string `json:"snapshot_id"`
}

func (h *Handler) Query(w http.ResponseWriter, r *http.Request) {
	var req queryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}
	if req.Question == "" {
		httputil.ErrResponse(w, http.StatusBadRequest, errBadRequest)
		return
	}

	claims, err := auth.ClaimsFromContext(r.Context())
	if err != nil {
		httputil.ErrResponse(w, http.StatusUnauthorized, err)
		return
	}

	clerkOrgID, err := auth.ResolveClerkOrgID(claims)
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}
	org, err := h.db.Queries.GetOrgByClerkID(r.Context(), clerkOrgID)
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, errors.New("org not found"))
		return
	}

	// Select first available AI provider key.
	var rawKey []byte
	var selectedProvider string
	for _, p := range providerOrder {
		k, kerr := h.db.Queries.GetAIProviderKey(r.Context(), store.GetAIProviderKeyParams{
			OrgID: org.ID, Provider: p,
		})
		if kerr != nil {
			continue
		}
		decrypted, derr := h.enc.Decrypt(k.KeyCipher)
		if derr == nil {
			rawKey = decrypted
			selectedProvider = p
			break
		}
	}
	if selectedProvider == "" {
		httputil.ErrResponse(w, http.StatusUnprocessableEntity, errNoAIKey)
		return
	}

	repoID, err := httputil.ParseUUID(r, "repoID")
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}

	repo, err := h.db.Queries.GetRepoByID(r.Context(), repoID)
	if err != nil {
		httputil.ErrResponse(w, http.StatusNotFound, errors.New("repo not found"))
		return
	}

	// Resolve snapshot.
	var snap store.Snapshot
	if req.SnapshotID != nil {
		snapID, perr := httputil.UUIDFromString(*req.SnapshotID)
		if perr != nil {
			httputil.ErrResponse(w, http.StatusBadRequest, perr)
			return
		}
		snap, err = h.db.Queries.GetSnapshotByID(r.Context(), snapID)
	} else {
		snap, err = h.db.Queries.GetLatestSnapshot(r.Context(), store.GetLatestSnapshotParams{
			RepoID: repoID, Branch: repo.DefaultBranch,
		})
	}
	if err != nil || snap.SnapshotVersion < 2 {
		httputil.ErrResponse(w, http.StatusUnprocessableEntity, errSnapshotOutdated)
		return
	}

	// Assemble context (Redis LRU or GitHub fetch).
	cacheKey := contextcache.Key(repoID.UUID.String(), snap.CommitSha)
	var contextMD string
	if cached, ok, _ := h.cache.Get(r.Context(), cacheKey); ok {
		contextMD = cached
	} else {
		var assembled strings.Builder
		metaBytes, _ := h.db.Queries.GetSnapshotByID(r.Context(), snap.ID) // already have snap
		// Download GCS snapshot for file_tree
		_ = metaBytes // handled below
		metaRaw, derr := h.downloadAndAssemble(r.Context(), repo, snap, &assembled)
		if derr != nil {
			h.log.Error("assemble context", zap.Error(derr))
			httputil.ErrResponse(w, http.StatusInternalServerError, errInternal)
			return
		}
		_ = metaRaw
		contextMD = assembled.String()
		go func() { _ = h.cache.Set(context.Background(), cacheKey, contextMD) }()
	}

	// Stream SSE.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	prompt := fmt.Sprintf("%s\n\n---\n\n%s\n\n---\n\nUser: %s", systemPrompt, contextMD, req.Question)
	if err := h.streamAI(r.Context(), w, selectedProvider, string(rawKey), prompt); err != nil {
		h.log.Error("stream ai", zap.Error(err), zap.String("provider", selectedProvider))
	}
}

func (h *Handler) downloadAndAssemble(ctx context.Context, repo store.Repository, snap store.Snapshot, buf *strings.Builder) ([]byte, error) {
	// This method is implemented alongside the context URL handler (Task 11).
	// Download GCS snapshot, parse file_tree, call assembly.Assemble.
	// Placeholder: implement by following the same logic in contexturl/context.go.
	return nil, fmt.Errorf("downloadAndAssemble: implement (see contexturl handler Task 11)")
}

// streamAI calls the selected provider and writes SSE chunks.
// Each chunk: data: {text}\n\n
// Anthropic: POST https://api.anthropic.com/v1/messages with stream:true
// OpenAI:    POST https://api.openai.com/v1/chat/completions with stream:true
func (h *Handler) streamAI(ctx context.Context, w http.ResponseWriter, provider, apiKey, prompt string) error {
	// TODO: implement per-provider streaming using net/http directly.
	// No external AI SDK needed — both Anthropic and OpenAI use chunked HTTP + SSE lines.
	return fmt.Errorf("streamAI: not implemented for %s", provider)
}

var (
	errBadRequest       = errors.New("question is required")
	errNoAIKey          = errors.New("no AI provider key configured — add one in Settings")
	errSnapshotOutdated = errors.New("re-run zigzag --upload to generate a v2 snapshot")
	errInternal         = errors.New("internal error")
)
```

> **Note:** Extract the GCS download + assembly logic into a shared function or service rather than duplicating it between `contexturl` and `query` handlers. A `api/internal/service/assembly/loader.go` that provides `LoadAndAssemble(ctx, gcsPath, repo, snapshot, cache, storage, github) (string, error)` is the right refactor — do this when both handlers are working.

- [ ] **Step 4: Run tests and compile**

```bash
cd api && go test ./internal/handler/query/... -v && go build ./...
```

- [ ] **Step 5: Commit**

```bash
git add api/internal/handler/query/
git commit -m "feat(handler): POST /api/v1/repos/{repoID}/query — SSE query console"
```

---

## Task 15: Wire Routes in main.go

**Files:**
- Modify: `api/cmd/main.go`

- [ ] **Step 1: Add imports**

```go
"encoding/base64"

contexturlhandler "github.com/LegationPro/zagforge/api/internal/handler/contexturl"
contexttokenshandler "github.com/LegationPro/zagforge/api/internal/handler/contexttokens"
aikeyshandler "github.com/LegationPro/zagforge/api/internal/handler/aikeys"
queryhandler "github.com/LegationPro/zagforge/api/internal/handler/query"
uploadhandler "github.com/LegationPro/zagforge/api/internal/handler/upload"
"github.com/LegationPro/zagforge/api/internal/cache/contextcache"
"github.com/LegationPro/zagforge/api/internal/middleware/clitoken"
"github.com/LegationPro/zagforge/api/internal/service/encryption"
storagepkg "github.com/LegationPro/zagforge/shared/go/storage"
```

- [ ] **Step 2: Construct GCS client (if not already in main.go)**

After the Redis block in `run()`:

```go
gcsClient, err := storagepkg.NewClient(context.Background(), storagepkg.Config{
    Bucket:   c.GCS.Bucket,
    Endpoint: c.GCS.Endpoint,
}, log)
if err != nil {
    return fmt.Errorf("create gcs client: %w", err)
}
```

> **Note:** Check if `gcsClient` is already constructed in `main.go` under a different variable name — the worker uses GCS, so a pattern may already exist. Do not duplicate.

- [ ] **Step 3: Construct encryption service and new handlers**

```go
encKeyBytes, err := base64.StdEncoding.DecodeString(c.App.EncryptionKeyBase64)
if err != nil {
    return fmt.Errorf("decode encryption key: %w", err)
}
encSvc, err := encryption.New(encKeyBytes)
if err != nil {
    return fmt.Errorf("init encryption: %w", err)
}

ctxCache := contextcache.NewRedis(rdb)

uploadH := uploadhandler.NewHandler(database, gcsClient, log)
contextURLH := contexturlhandler.NewHandler(database, ctxCache, ch, gcsClient, log)
ctxTokensH := contexttokenshandler.NewHandler(database, log)
aiKeysH := aikeyshandler.NewHandler(database, encSvc, log)
queryH := queryhandler.NewHandler(database, ctxCache, ch, encSvc, log)
```

- [ ] **Step 4: Register routes**

After the existing `v1` route group block:

```go
// Context URL — public (no auth), per-token rate limit applied inside handler.
// Uses router.HEAD which requires Task 3 (HEAD method added to shared router).
contextURLRoutes := r.Group()
if err := contextURLRoutes.Create([]router.Subroute{
    {Method: router.HEAD, Path: "/v1/context/{token}", Handler: contextURLH.Head},
    {Method: router.GET,  Path: "/v1/context/{token}", Handler: contextURLH.Get},
}); err != nil {
    return fmt.Errorf("register context URL routes: %w", err)
}

// CLI upload — CLI token auth, no Clerk JWT.
uploadRoutes := r.Group()
uploadRoutes.Use(contenttype.RequireJSON())
uploadRoutes.Use(clitoken.Auth(c.App.CLIAPIKey))
if err := uploadRoutes.Create([]router.Subroute{
    {Method: router.POST, Path: "/api/v1/upload", Handler: uploadH.Upload},
}); err != nil {
    return fmt.Errorf("register upload routes: %w", err)
}

// Phase 5 Clerk-authenticated routes (same auth + rate limit as existing v1 group).
v5 := r.Group()
v5.Use(auth.Auth(log))
v5.Use(ratelimit.RateLimit(rdb, ratelimit.RateLimitConfig{
    MaxRequests: 60,
    Window:      1 * time.Minute,
}, "api", log))
if err := v5.Create([]router.Subroute{
    {Method: router.GET,    Path: "/api/v1/repos/{repoID}/context-tokens",              Handler: ctxTokensH.List},
    {Method: router.POST,   Path: "/api/v1/repos/{repoID}/context-tokens",              Handler: ctxTokensH.Create},
    {Method: router.DELETE, Path: "/api/v1/repos/{repoID}/context-tokens/{tokenID}",    Handler: ctxTokensH.Delete},
    {Method: router.GET,    Path: "/api/v1/repos/{repoID}/query",                       Handler: queryH.Query},
    {Method: router.GET,    Path: "/api/v1/settings/ai-keys",                           Handler: aiKeysH.List},
    {Method: router.PUT,    Path: "/api/v1/settings/ai-keys",                           Handler: aiKeysH.Upsert},
    {Method: router.DELETE, Path: "/api/v1/settings/ai-keys/{provider}",                Handler: aiKeysH.Delete},
}); err != nil {
    return fmt.Errorf("register phase 5 routes: %w", err)
}
```

Note: query route uses POST not GET — fix the method above to `router.POST`.

- [ ] **Step 5: Build**

```bash
cd api && go build ./...
```

Expected: no errors.

- [ ] **Step 6: Smoke test locally**

```bash
task dev   # start local stack

curl -s localhost:8080/livez
# expected: {"data":{"status":"ok"}} or similar

curl -v localhost:8080/v1/context/zf_ctx_doesnotexist
# expected: 404 JSON error

curl -v -X POST localhost:8080/api/v1/upload \
  -H "Authorization: Bearer zf_pk_devtestkey1234567890" \
  -H "Content-Type: application/json" \
  -d '{"org_slug":"test"}'
# expected: 400 (missing fields) — not 401 or 500
```

- [ ] **Step 7: Commit**

```bash
git add api/cmd/main.go
git commit -m "feat(main): wire phase 5 routes — upload, context URL, query console, token + AI key mgmt"
```

---

## Task 16: Integration Test

**Files:**
- Create: `api/internal/integration/phase5_test.go`

- [ ] **Step 1: Check existing integration helpers**

Read `api/internal/integration/helpers_test.go` to understand `testServerURL(t)` and auth helpers. Reuse the same patterns.

- [ ] **Step 2: Write the test**

```go
// api/internal/integration/phase5_test.go
//go:build integration

package integration_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"testing"
)

// TestCLIUploadReturns201 posts a minimal v2 snapshot with a valid CLI key.
func TestCLIUploadReturns201(t *testing.T) {
	baseURL := testServerURL(t)
	cliKey := os.Getenv("CLI_API_KEY")
	if cliKey == "" {
		t.Skip("CLI_API_KEY not set")
	}

	payload := map[string]any{
		"org_slug":       "test-org",
		"repo_full_name": "test-org/test-repo",
		"commit_sha":     "abc1234567890123456789012345678901234567",
		"branch":         "main",
		"metadata_snapshot": map[string]any{
			"snapshot_version": 2,
			"zigzag_version":   "0.12.0",
			"commit_sha":       "abc1234567890123456789012345678901234567",
			"branch":           "main",
			"summary":          map[string]any{"source_files": 1, "total_lines": 5},
			"file_tree":        []map[string]any{{"path": "main.go", "language": "go", "lines": 5, "sha": "blobsha"}},
		},
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, baseURL+"/api/v1/upload", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cliKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("upload: got %d, want 201", resp.StatusCode)
	}
}

// TestContextURLNotFound verifies unknown tokens return 404.
func TestContextURLNotFound(t *testing.T) {
	baseURL := testServerURL(t)
	resp, err := http.Get(baseURL + "/v1/context/zf_ctx_doesnotexist123456789012345")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("got %d, want 404", resp.StatusCode)
	}
}
```

- [ ] **Step 3: Run integration tests**

```bash
cd api && go test ./internal/integration/... -tags=integration -v -run TestCLIUpload
cd api && go test ./internal/integration/... -tags=integration -v -run TestContextURLNotFound
```

Expected: PASS (or SKIP if local stack not running).

- [ ] **Step 4: Commit**

```bash
git add api/internal/integration/phase5_test.go
git commit -m "test(integration): phase 5 CLI upload + context URL round-trip"
```

---

## Done

All Phase 5 backend work is complete when:

- [ ] `go build ./...` passes with no errors across all modules
- [ ] `go test ./...` passes (unit tests only)
- [ ] `go test ./... -tags=integration` passes against local stack
- [ ] `GET /livez` returns 200
- [ ] `POST /api/v1/upload` with `Authorization: Bearer <CLI_API_KEY>` returns 201 for a valid v2 payload
- [ ] `GET /v1/context/zf_ctx_doesnotexist` returns 404
- [ ] `POST /api/v1/repos/{repoID}/context-tokens` with Clerk JWT returns 201 with `raw_token`
- [ ] `GET /api/v1/settings/ai-keys` with Clerk JWT returns 200

**Next plans:**
- `2026-03-21-phase5-dashboard.md` — `apps/cloud` Next.js dashboard (zigzag-web monorepo)
- `2026-03-21-phase5-cli.md` — `zigzag --upload` flag (zigzag CLI repo)
