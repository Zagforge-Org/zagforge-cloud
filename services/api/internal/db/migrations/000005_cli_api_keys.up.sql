CREATE TABLE cli_api_keys (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID REFERENCES users(id) ON DELETE CASCADE,
    org_id     UUID REFERENCES organizations(id) ON DELETE CASCADE,
    key_hash   TEXT UNIQUE NOT NULL,
    key_hint   TEXT NOT NULL,
    label      TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT cli_api_keys_owner_check CHECK (
        (user_id IS NOT NULL AND org_id IS NULL) OR
        (user_id IS NULL AND org_id IS NOT NULL)
    )
);

CREATE INDEX idx_cli_api_keys_org ON cli_api_keys (org_id) WHERE org_id IS NOT NULL;
CREATE INDEX idx_cli_api_keys_user ON cli_api_keys (user_id) WHERE user_id IS NOT NULL;
