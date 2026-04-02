
-- CLI uploads have no associated job; make job_id nullable.
-- DROP job_id from snapshots
ALTER TABLE snapshots ALTER COLUMN job_id DROP NOT NULL;

-- Store ignore_patterns and zigzag_config_hash from the Zigzag run.
-- Add JSONB metadata to snapshots.
ALTER TABLE snapshots ADD COLUMN metadata JSONB;

-- Encrypted AI provider API keys, one per provider per org.
-- Include popular ai providers
-- OpenAI (ChatGPT)
-- Anthropic (Claude)
-- Google (Gemini)
-- xAI (Grok)
CREATE TABLE ai_provider_keys (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id      UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    provider    TEXT NOT NULL CHECK (provider IN ('openai', 'anthropic', 'google', 'xai')),
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

-- Make token_hash indexable for faster lookups.
CREATE INDEX idx_context_tokens_hash ON context_tokens (token_hash);
