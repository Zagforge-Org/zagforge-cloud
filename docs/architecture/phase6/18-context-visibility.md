# 18 — Context Token Visibility Modes

**Phase:** 6
**Status:** Planned
**Depends on:** Phase 5 backend (context tokens, context URL) + Phase 5 frontend (dashboard)

---

## Problem

Context tokens are currently public-only — anyone with the token string can access the assembled codebase snapshot. This is secure (192-bit entropy, SHA-256 hashed, revocable, expirable) but some organizations need tighter access control for sensitive codebases.

## Solution

Add a `visibility` field to context tokens with three modes:

| Mode | Who can access | Auth required | Use case |
|---|---|---|---|
| **Public** (default) | Anyone with the token | None | AI tools, external collaborators |
| **Private** | Org members or account owner only | Zitadel OIDC JWT | Internal team use |
| **Protected** | Invited users only | Zitadel OIDC JWT + allowlist | Selective sharing with contractors/partners |

---

## Data Model

### Migration: `000003_context_visibility.up.sql`

```sql
-- Add visibility column with default 'public' (backwards compatible).
CREATE TYPE context_visibility AS ENUM ('public', 'private', 'protected');
ALTER TABLE context_tokens ADD COLUMN visibility context_visibility NOT NULL DEFAULT 'public';

-- Allowlist for protected tokens.
CREATE TABLE context_token_allowed_users (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    token_id   UUID NOT NULL REFERENCES context_tokens(id) ON DELETE CASCADE,
    zitadel_user_id TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (token_id, zitadel_user_id)
);

CREATE INDEX idx_context_token_allowed_users_token ON context_token_allowed_users (token_id);
```

### Down migration

```sql
DROP TABLE IF EXISTS context_token_allowed_users;
ALTER TABLE context_tokens DROP COLUMN IF EXISTS visibility;
DROP TYPE IF EXISTS context_visibility;
```

---

## API Changes

### Context Token CRUD

`POST /api/v1/repos/{repoID}/context-tokens` — add optional fields:

```json
{
  "label": "cursor-rules",
  "visibility": "protected",
  "allowed_users": ["user_2abc123", "user_2def456"]
}
```

`GET /api/v1/repos/{repoID}/context-tokens` — response includes:

```json
{
  "id": "...",
  "visibility": "protected",
  "allowed_user_count": 2
}
```

### New endpoint

`PUT /api/v1/repos/{repoID}/context-tokens/{tokenID}/allowed-users` — manage allowlist:

```json
{
  "zitadel_user_ids": ["user_2abc123", "user_2def456"]
}
```

### Context URL behavior changes

`HEAD /v1/context/{token}` and `GET /v1/context/{token}`:

| Visibility | No JWT | Valid JWT, same org | Valid JWT, different org | Valid JWT, in allowlist |
|---|---|---|---|---|
| Public | 200 | 200 | 200 | 200 |
| Private | 401 | 200 | 403 | 403 |
| Protected | 401 | 403 | 403 | 200 |

For private/protected tokens, the context URL handler must:
1. Check `token.visibility`
2. If not `public`, require `Authorization: Bearer <zitadel_jwt>` header
3. Verify Zitadel JWT and extract user identity
4. Private: verify user owns the token (personal) or is a member of `token.org_id` (org)
5. Protected: verify user's Zitadel ID is in `context_token_allowed_users`

---

## Handler Logic

```
GET /v1/context/{token}
  ├─ Look up token by hash
  ├─ Check expiration
  ├─ If visibility == "public" → proceed (current behavior)
  ├─ If visibility == "private"
  │   ├─ Require Zitadel JWT
  │   ├─ If token.user_id is set → verify JWT sub == token owner
  │   └─ If token.org_id is set → verify user is org member → proceed or 403
  └─ If visibility == "protected"
      ├─ Require Zitadel JWT
      ├─ Extract zitadel_user_id from JWT sub claim
      └─ Query context_token_allowed_users → proceed or 403
```

---

## Migration Path

- Default visibility is `public` — all existing tokens continue working with no changes
- New tokens default to `public` unless specified
- Dashboard UI shows visibility selector on token creation
- CLI `zigzag --upload` always creates public tokens (no interactive auth)

---

## Implementation Order

1. Database migration (add column + allowlist table)
2. sqlc queries (CRUD for allowed users, update token visibility)
3. Update context token Create handler (accept visibility + allowed_users)
4. Update context URL handler (check visibility before serving)
5. Dashboard UI (visibility selector, allowlist management)
6. Tests (unit + integration for all three modes)
