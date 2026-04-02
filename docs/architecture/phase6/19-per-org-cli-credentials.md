# 19 — Per-Org CLI Credentials

**Phase:** 6
**Status:** Planned
**Depends on:** Phase 5 backend (CLI upload endpoint)

---

## Problem

The CLI upload endpoint (`POST /api/v1/upload`) uses a single global static API key (`CLI_API_KEY`). Anyone with this key can upload snapshots to any organization. The key doesn't encode which org it belongs to — the org is specified in the request body (`org_slug`), which is trusted without verification.

## Solution

Replace the global static key with per-org CLI credentials. Each org gets its own API key that is bound to that org at creation time. The upload handler validates that the key's org matches the requested `org_slug`.

### Options

| Approach | Complexity | Security | UX |
|---|---|---|---|
| **A: DB-backed API keys** (recommended) | Medium | Strong — revocable, per-user/org, auditable | User or org admin generates key in dashboard |
| B: HMAC-signed org tokens | Low | Good — encodes org_id, verifiable without DB | No revocation without key rotation |
| C: Zitadel OIDC from CLI | Low | Strong — leverages existing auth | Requires browser OAuth flow from CLI |

### Recommended: Option A

**New table:**
```sql
CREATE TABLE cli_api_keys (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID REFERENCES users(id) ON DELETE CASCADE,
    org_id     UUID REFERENCES organizations(id) ON DELETE CASCADE,
    key_hash   TEXT UNIQUE NOT NULL,
    key_hint   TEXT NOT NULL,
    label      TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (
        (user_id IS NOT NULL AND org_id IS NULL) OR
        (user_id IS NULL AND org_id IS NOT NULL)
    )
);
```

**Flow:**
1. User or org admin generates a CLI key in dashboard → `POST /api/v1/settings/cli-keys` (personal) or `POST /api/v1/orgs/{orgID}/settings/cli-keys` (org)
2. Raw key returned once (e.g., `zf_cli_<base64>`)
3. Key hash stored in DB (SHA-256, same pattern as context tokens)
4. CLI sends `Authorization: Bearer zf_cli_<base64>` on upload
5. Middleware hashes the key, looks up in DB, extracts `user_id` or `org_id`
6. Upload handler verifies the resolved scope matches the target in the request body

**Migration path:**
- Keep `CLI_API_KEY` env var as a fallback during transition
- New per-org keys take precedence
- Deprecate global key after all orgs migrate

---

## Implementation Order

1. Database migration (cli_api_keys table)
2. sqlc queries (insert, lookup by hash, list by org, delete)
3. CLI key CRUD handler (`/api/v1/orgs/settings/cli-keys`)
4. Update clitoken middleware to support both global key and per-org DB lookup
5. Upload handler validates key's org matches request org_slug
6. Dashboard UI for key management
7. Deprecate global CLI_API_KEY
