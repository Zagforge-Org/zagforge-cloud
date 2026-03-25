-- Migration: Auth migration (Clerk → Zitadel)
-- Adds users, memberships, sessions, audit_log tables.
-- Converts organizations from Clerk to Zitadel.
-- Adds dual ownership (user_id / org_id) to resource tables.

-- ============================================================
-- Users
-- ============================================================

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

-- ============================================================
-- 2. Organizations: clerk_org_id → zitadel_org_id
-- ============================================================

ALTER TABLE organizations DROP CONSTRAINT organizations_clerk_org_id_key;
ALTER TABLE organizations DROP COLUMN clerk_org_id;
ALTER TABLE organizations ADD COLUMN zitadel_org_id TEXT UNIQUE NOT NULL;

-- ============================================================
-- Repositories: add dual ownership
-- ============================================================

-- Drop the existing NOT NULL constraint on org_id
ALTER TABLE repositories ALTER COLUMN org_id DROP NOT NULL;

-- Add user_id column
ALTER TABLE repositories ADD COLUMN user_id UUID REFERENCES users(id) ON DELETE CASCADE;

-- Enforce exactly one owner
ALTER TABLE repositories ADD CONSTRAINT repositories_owner_check CHECK (
    (user_id IS NOT NULL AND org_id IS NULL) OR
    (user_id IS NULL AND org_id IS NOT NULL)
);

CREATE INDEX idx_repositories_user_id ON repositories (user_id);

-- ============================================================
-- AI Provider Keys: add dual ownership
-- ============================================================

-- Drop existing unique constraint
ALTER TABLE ai_provider_keys DROP CONSTRAINT ai_provider_keys_org_id_provider_key;

-- Make org_id nullable
ALTER TABLE ai_provider_keys ALTER COLUMN org_id DROP NOT NULL;

-- Add user_id column
ALTER TABLE ai_provider_keys ADD COLUMN user_id UUID REFERENCES users(id) ON DELETE CASCADE;

-- Enforce exactly one owner
ALTER TABLE ai_provider_keys ADD CONSTRAINT ai_provider_keys_owner_check CHECK (
    (user_id IS NOT NULL AND org_id IS NULL) OR
    (user_id IS NULL AND org_id IS NOT NULL)
);

-- One key per provider per owner (partial unique indexes)
CREATE UNIQUE INDEX idx_ai_provider_keys_user_provider
    ON ai_provider_keys (user_id, provider) WHERE user_id IS NOT NULL;
CREATE UNIQUE INDEX idx_ai_provider_keys_org_provider
    ON ai_provider_keys (org_id, provider) WHERE org_id IS NOT NULL;

-- ============================================================
-- Context Tokens: add dual ownership
-- ============================================================

-- Make org_id nullable
ALTER TABLE context_tokens ALTER COLUMN org_id DROP NOT NULL;

-- Add user_id column
ALTER TABLE context_tokens ADD COLUMN user_id UUID REFERENCES users(id) ON DELETE CASCADE;

-- Enforce exactly one owner
ALTER TABLE context_tokens ADD CONSTRAINT context_tokens_owner_check CHECK (
    (user_id IS NOT NULL AND org_id IS NULL) OR
    (user_id IS NULL AND org_id IS NOT NULL)
);

-- ============================================================
-- Memberships
-- ============================================================

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

-- ============================================================
-- Sessions
-- ============================================================

CREATE TABLE sessions (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id            UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    zitadel_session_id TEXT UNIQUE NOT NULL,
    device_name        TEXT,
    ip_address         INET,
    last_active_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_sessions_user_id ON sessions (user_id);

-- ============================================================
-- Audit Log
-- ============================================================

CREATE TABLE audit_log (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID REFERENCES users(id),
    org_id      UUID REFERENCES organizations(id) ON DELETE CASCADE,
    actor_id    UUID NOT NULL REFERENCES users(id),
    action      TEXT NOT NULL,
    target_id   UUID,
    metadata    JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (
        (user_id IS NOT NULL AND org_id IS NULL) OR
        (user_id IS NULL AND org_id IS NOT NULL)
    )
);

CREATE INDEX idx_audit_log_org_id ON audit_log (org_id, created_at DESC);
CREATE INDEX idx_audit_log_user_id ON audit_log (user_id, created_at DESC);
