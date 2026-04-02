-- Reusable trigger function.
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- ============================================================
-- USERS
-- ============================================================
CREATE TABLE users (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email              TEXT UNIQUE NOT NULL,
    email_verified     BOOLEAN NOT NULL DEFAULT false,
    first_name         TEXT,
    last_name          TEXT,
    nickname           TEXT,
    bio                TEXT,
    country            TEXT,
    age                INT,
    timezone           TEXT NOT NULL DEFAULT 'UTC',
    language           TEXT NOT NULL DEFAULT 'en',
    avatar_url         TEXT,
    phone_cipher       BYTEA,
    social_links       JSONB NOT NULL DEFAULT '{}',
    profile_visibility TEXT NOT NULL DEFAULT 'team_only'
                       CHECK (profile_visibility IN ('public', 'private', 'team_only')),
    onboarding_step    TEXT NOT NULL DEFAULT 'profile'
                       CHECK (onboarding_step IN ('profile', 'connect_oauth', 'join_team', 'completed')),
    is_platform_admin  BOOLEAN NOT NULL DEFAULT false,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TRIGGER users_set_updated_at
BEFORE UPDATE ON users
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ============================================================
-- OAUTH IDENTITIES
-- ============================================================
CREATE TABLE oauth_identities (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id          UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider         TEXT NOT NULL CHECK (provider IN ('github', 'google')),
    provider_id      TEXT NOT NULL,
    email            TEXT,
    display_name     TEXT,
    avatar_url       TEXT,
    access_token     BYTEA,
    refresh_token    BYTEA,
    token_expires_at TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (provider, provider_id)
);

CREATE INDEX idx_oauth_identities_user_id ON oauth_identities (user_id);

CREATE TRIGGER oauth_identities_set_updated_at
BEFORE UPDATE ON oauth_identities
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ============================================================
-- SESSIONS
-- ============================================================
CREATE TABLE sessions (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id            UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    ip_address         INET,
    user_agent         TEXT,
    device_name        TEXT,
    device_fingerprint TEXT,
    country            TEXT,
    last_active_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at         TIMESTAMPTZ NOT NULL,
    revoked_at         TIMESTAMPTZ,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_sessions_user_id ON sessions (user_id);
CREATE INDEX idx_sessions_active ON sessions (user_id, revoked_at) WHERE revoked_at IS NULL;

-- ============================================================
-- REFRESH TOKENS
-- ============================================================
CREATE TABLE refresh_tokens (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    session_id UUID NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    token_hash TEXT UNIQUE NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens (user_id);
CREATE INDEX idx_refresh_tokens_session_id ON refresh_tokens (session_id);

-- ============================================================
-- OAUTH STATES (CSRF protection)
-- ============================================================
CREATE TABLE oauth_states (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    state        TEXT UNIQUE NOT NULL,
    provider     TEXT NOT NULL,
    redirect_uri TEXT NOT NULL,
    expires_at   TIMESTAMPTZ NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_oauth_states_expires ON oauth_states (expires_at);

-- ============================================================
-- FAILED LOGIN ATTEMPTS
-- ============================================================
CREATE TABLE failed_login_attempts (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    identifier TEXT NOT NULL,
    ip_address INET NOT NULL,
    user_agent TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_failed_logins_identifier ON failed_login_attempts (identifier, created_at DESC);
CREATE INDEX idx_failed_logins_ip ON failed_login_attempts (ip_address, created_at DESC);

-- ============================================================
-- MFA SETTINGS
-- ============================================================
CREATE TABLE mfa_settings (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID UNIQUE NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    totp_secret     BYTEA,
    totp_enabled    BOOLEAN NOT NULL DEFAULT false,
    totp_verified_at TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TRIGGER mfa_settings_set_updated_at
BEFORE UPDATE ON mfa_settings
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ============================================================
-- MFA BACKUP CODES
-- ============================================================
CREATE TABLE mfa_backup_codes (
    id        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id   UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code_hash TEXT NOT NULL,
    used_at   TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_mfa_backup_codes_user_id ON mfa_backup_codes (user_id);

-- ============================================================
-- ORGANIZATIONS
-- ============================================================
CREATE TABLE organizations (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug          TEXT UNIQUE NOT NULL,
    name          TEXT NOT NULL,
    logo_url      TEXT,
    plan          TEXT NOT NULL DEFAULT 'free'
                  CHECK (plan IN ('free', 'pro', 'enterprise')),
    billing_email TEXT,
    max_members   INT NOT NULL DEFAULT 5,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TRIGGER organizations_set_updated_at
BEFORE UPDATE ON organizations
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ============================================================
-- ORG MEMBERSHIPS
-- ============================================================
CREATE TABLE org_memberships (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id     UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role       TEXT NOT NULL DEFAULT 'member'
               CHECK (role IN ('owner', 'admin', 'member')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (org_id, user_id)
);

CREATE INDEX idx_org_memberships_user_id ON org_memberships (user_id);
CREATE INDEX idx_org_memberships_org_id ON org_memberships (org_id);

CREATE TRIGGER org_memberships_set_updated_at
BEFORE UPDATE ON org_memberships
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ============================================================
-- TEAMS
-- ============================================================
CREATE TABLE teams (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id      UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    slug        TEXT NOT NULL,
    name        TEXT NOT NULL,
    description TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (org_id, slug)
);

CREATE TRIGGER teams_set_updated_at
BEFORE UPDATE ON teams
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ============================================================
-- TEAM MEMBERSHIPS
-- ============================================================
CREATE TABLE team_memberships (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id    UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role       TEXT NOT NULL DEFAULT 'member'
               CHECK (role IN ('lead', 'member')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (team_id, user_id)
);

CREATE INDEX idx_team_memberships_user_id ON team_memberships (user_id);

-- ============================================================
-- INVITES
-- ============================================================
CREATE TABLE invites (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id      UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    team_id     UUID REFERENCES teams(id) ON DELETE SET NULL,
    invited_by  UUID NOT NULL REFERENCES users(id),
    email       TEXT NOT NULL,
    role        TEXT NOT NULL DEFAULT 'member'
                CHECK (role IN ('admin', 'member')),
    token_hash  TEXT UNIQUE NOT NULL,
    status      TEXT NOT NULL DEFAULT 'pending'
                CHECK (status IN ('pending', 'accepted', 'revoked', 'expired')),
    expires_at  TIMESTAMPTZ NOT NULL,
    accepted_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_invites_org_id ON invites (org_id);
CREATE INDEX idx_invites_email ON invites (email);

-- ============================================================
-- AUDIT LOGS
-- ============================================================
CREATE TABLE audit_logs (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id      UUID REFERENCES organizations(id) ON DELETE SET NULL,
    actor_id    UUID REFERENCES users(id) ON DELETE SET NULL,
    action      TEXT NOT NULL,
    target_type TEXT,
    target_id   UUID,
    ip_address  INET,
    user_agent  TEXT,
    metadata    JSONB NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_audit_logs_org_id ON audit_logs (org_id, created_at DESC);
CREATE INDEX idx_audit_logs_actor_id ON audit_logs (actor_id, created_at DESC);
CREATE INDEX idx_audit_logs_action ON audit_logs (action, created_at DESC);

-- ============================================================
-- WEBHOOK SUBSCRIPTIONS
-- ============================================================
CREATE TABLE webhook_subscriptions (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id     UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    url        TEXT NOT NULL,
    secret     TEXT NOT NULL,
    events     TEXT[] NOT NULL,
    active     BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TRIGGER webhook_subscriptions_set_updated_at
BEFORE UPDATE ON webhook_subscriptions
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ============================================================
-- WEBHOOK DELIVERIES
-- ============================================================
CREATE TABLE webhook_deliveries (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subscription_id UUID NOT NULL REFERENCES webhook_subscriptions(id) ON DELETE CASCADE,
    event           TEXT NOT NULL,
    payload         JSONB NOT NULL,
    response_status INT,
    response_body   TEXT,
    delivered_at    TIMESTAMPTZ,
    attempts        INT NOT NULL DEFAULT 0,
    next_retry_at   TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_webhook_deliveries_pending ON webhook_deliveries (next_retry_at)
    WHERE delivered_at IS NULL;
