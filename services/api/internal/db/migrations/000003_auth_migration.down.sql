-- Reverse: Auth migration (Zitadel → Clerk)

DROP TABLE IF EXISTS audit_log;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS memberships;

-- Context tokens: remove dual ownership
ALTER TABLE context_tokens DROP CONSTRAINT IF EXISTS context_tokens_owner_check;
ALTER TABLE context_tokens DROP COLUMN IF EXISTS user_id;
ALTER TABLE context_tokens ALTER COLUMN org_id SET NOT NULL;

-- AI provider keys: remove dual ownership, restore original unique constraint
DROP INDEX IF EXISTS idx_ai_provider_keys_user_provider;
DROP INDEX IF EXISTS idx_ai_provider_keys_org_provider;
ALTER TABLE ai_provider_keys DROP CONSTRAINT IF EXISTS ai_provider_keys_owner_check;
ALTER TABLE ai_provider_keys DROP COLUMN IF EXISTS user_id;
ALTER TABLE ai_provider_keys ALTER COLUMN org_id SET NOT NULL;
ALTER TABLE ai_provider_keys ADD CONSTRAINT ai_provider_keys_org_id_provider_key UNIQUE (org_id, provider);

-- Repositories: remove dual ownership
DROP INDEX IF EXISTS idx_repositories_user_id;
ALTER TABLE repositories DROP CONSTRAINT IF EXISTS repositories_owner_check;
ALTER TABLE repositories DROP COLUMN IF EXISTS user_id;
ALTER TABLE repositories ALTER COLUMN org_id SET NOT NULL;

-- Organizations: zitadel_org_id → clerk_org_id
ALTER TABLE organizations DROP COLUMN IF EXISTS zitadel_org_id;
ALTER TABLE organizations ADD COLUMN clerk_org_id TEXT UNIQUE NOT NULL;

-- Users
DROP TRIGGER IF EXISTS users_set_updated_at ON users;
DROP TABLE IF EXISTS users;
