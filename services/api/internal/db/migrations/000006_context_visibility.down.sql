DROP TABLE IF EXISTS context_token_allowed_users;
ALTER TABLE context_tokens DROP COLUMN IF EXISTS visibility;
DROP TYPE IF EXISTS context_visibility;
