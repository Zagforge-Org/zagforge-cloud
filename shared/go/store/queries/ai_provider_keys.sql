-- name: UpsertAIProviderKey :one
INSERT INTO ai_provider_keys (org_id, provider, key_cipher, key_hint)
VALUES ($1, $2, $3, $4)
ON CONFLICT (org_id, provider) DO UPDATE
    SET key_cipher = EXCLUDED.key_cipher,
    key_hint = EXCLUDED.key_hint
RETURNING id, org_id, provider, key_cipher, key_hint,
created_at;

-- name: GetAIProviderKey :one
SELECT id, org_id, provider, key_cipher, key_hint, created_at
FROM ai_provider_keys
WHERE org_id = $1 AND provider = $2;

-- name: ListAIProviderKeys :many
SELECT id, org_id, provider, key_hint, created_at
FROM ai_provider_keys
WHERE org_id = $1
ORDER BY provider ASC;

-- name: DeleteAIProviderKey :exec
DELETE FROM ai_provider_keys WHERE org_id = $1 AND provider = $2;
