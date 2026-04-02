# Zagforge — Context Proxy [Phase 5]

The Context Proxy is the core Phase 5 feature. It turns a Zagforge repo into a persistent, always-current URL that any AI tool can fetch. The developer uploads once; every subsequent fetch reflects the latest snapshot automatically.

**Zero-trust guarantee:** File contents are never stored in Zagforge. GCS holds only the file map (paths + SHAs). Actual file content is fetched from GitHub at query time into Go process memory and never written to disk, GCS, or Postgres.

---

## Context URL

```
GET /v1/context/{raw_token}
```

A public, unauthenticated endpoint. The `raw_token` in the path is the credential.

**Token format:** `zf_ctx_<32 random URL-safe chars>` (~192 bits entropy). Raw token returned once at creation; only the SHA-256 hash is stored in `context_tokens`.

**Token types (controlled by `target_snapshot_id`):**
- `NULL` → live token: always resolves to the latest snapshot for the repo
- set → locked token: always resolves to a specific snapshot version (useful for stable docs, release branches)

**Response codes:**
| Code | Condition |
|------|-----------|
| 200 | Token valid, context assembled |
| 404 | Token not found |
| 410 Gone | Token found but `expires_at < now()` — 410 not 401 to avoid oracle attacks |
| 422 | Snapshot is v1 format (no `file_tree`); user must re-upload |

**CORS:** `Access-Control-Allow-Origin: *` — callers are AI tools (Cursor, Claude Projects), not browser origins.

**Rate limiting:** 60 req/min per token hash. Redis key: `rl:ctx:{token_hash}`.

---

## Assembly Pipeline

When a context URL is fetched and the assembled markdown is not cached, the Go backend performs streaming assembly:

```
1. Flush markdown header immediately
   → "# Codebase Snapshot — {repo_full_name} @ {commit_sha}\n\n"
   → User/tool sees content start instantly; no spinner wait

2. For each file in file_tree (from GCS snapshot.json):
   a. Fetch file content from GitHub via installation token
      → GET /repos/{owner}/{repo}/git/blobs/{sha} (blob API, by SHA)
      → Respects ignore_patterns from snapshot.metadata
   b. Wrap in markdown code block with file path header
   c. Flush to response stream

3. Store assembled result in Redis
   → Key: ctx:{repo_id}:{commit_sha}
   → TTL: 10 minutes
   → Subsequent requests within TTL skip GitHub fetch entirely
```

**Why streaming matters:** A large repo may have 50+ files. Streaming means the AI tool starts consuming context while the backend is still fetching — no 4-second wait staring at a spinner.

**Why blob SHA lookup:** Using the file SHA from `file_tree` (rather than fetching by branch + path) is exact and idempotent. It fetches the precise version of the file that was present when Zigzag ran, not whatever is on the branch tip at query time.

---

## HEAD Endpoint

```
HEAD /v1/context/{raw_token}
```

Lightweight token validation. Returns the same 404/410 codes as GET. On success:

```
200 OK
X-Snapshot-ID: <uuid>
X-Commit-SHA: <sha>
Content-Type: text/markdown
```

No GitHub fetch is triggered. Used by AI tools and curl-based health checks to verify a token is alive and to read the snapshot metadata before downloading the full context.

---

## Context Token Lifecycle

```
POST /api/v1/{org}/{repo}/context-tokens
  Body: { label?: string, expires_at?: timestamp, target_snapshot_id?: uuid }
  Returns: { id, raw_token (once only), label, created_at }

GET /api/v1/{org}/{repo}/context-tokens
  Returns: [{ id, label, last_used_at, expires_at, target_snapshot_id }]
  Never returns raw_token or token_hash

DELETE /api/v1/{org}/{repo}/context-tokens/{id}
  Deletes row → token immediately invalid
```

The raw token is returned **only once** at creation. After that it cannot be recovered — the user must create a new token. This mirrors the GitHub PAT and Stripe secret key UX.

---

## Copy Buttons (Dashboard UX)

The Context URL panel in `apps/cloud` exposes two copy actions:

**Copy for Cursor** — copies:
```
# .cursorrules
fetch: https://api.zagforge.com/v1/context/zf_ctx_...
```

**Copy for Claude** — copies:
```
Fetch the following URL at the start of our conversation for codebase context:
https://api.zagforge.com/v1/context/zf_ctx_...
```

These are pre-formatted clipboard payloads — no manual formatting required from the user.

---

## Query Console

The Query Console is the dashboard-embedded AI chat, powered by the same assembly pipeline as the Context URL.

```
POST /api/v1/{org}/{repo}/query
Auth: Zitadel OIDC JWT
Body: { question: string, snapshot_id?: uuid }
Response: text/event-stream (SSE)
```

**Provider selection (Phase 5, hardcoded):** Anthropic → OpenAI → Google (first configured key wins).

**System prompt (Phase 5, hardcoded):**
```
You are an expert on this codebase. Refer to files by their full path.
Answer concisely and precisely. If unsure, say so.
```

User-editable system prompts and multi-model toggles are deferred to a future phase.

**SSE format:** Each chunk is a standard `data: {text}\n\n` event. The browser `EventSource` or custom `useSSE` hook in `apps/cloud/lib/sse.ts` consumes this and appends to the message bubble.
