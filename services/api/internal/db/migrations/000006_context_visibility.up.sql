-- Add visibility column with default 'public' (backwards compatible).
CREATE TYPE context_visibility AS ENUM ('public', 'private', 'protected');
ALTER TABLE context_tokens ADD COLUMN visibility context_visibility NOT NULL DEFAULT 'public';

-- Allowlist for protected tokens.
CREATE TABLE context_token_allowed_users (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    token_id   UUID NOT NULL REFERENCES context_tokens(id) ON DELETE CASCADE,
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (token_id, user_id)
);

CREATE INDEX idx_context_token_allowed_users_token ON context_token_allowed_users (token_id);
