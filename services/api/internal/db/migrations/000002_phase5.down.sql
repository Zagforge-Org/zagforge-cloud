
DROP TABLE IF EXISTS context_tokens;
DROP TABLE IF EXISTS ai_provider_keys;
ALTER TABLE snapshots DROP COLUMN IF EXISTS metadata;
ALTER TABLE snapshots ALTER COLUMN job_id SET NOT NULL;
