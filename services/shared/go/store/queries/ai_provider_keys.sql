-- name: UpsertAIProviderKeyForOrg :one
INSERT INTO ai_provider_keys (org_id, provider, key_cipher, key_hint)
VALUES ($1, $2, $3, $4)
ON CONFLICT (org_id, provider) WHERE org_id IS NOT NULL DO UPDATE
    SET key_cipher = EXCLUDED.key_cipher,
    key_hint = EXCLUDED.key_hint
RETURNING id, user_id, org_id, provider, key_cipher, key_hint, created_at;

-- name: UpsertAIProviderKeyForUser :one
INSERT INTO ai_provider_keys (user_id, provider, key_cipher, key_hint)
VALUES ($1, $2, $3, $4)
ON CONFLICT (user_id, provider) WHERE user_id IS NOT NULL DO UPDATE
    SET key_cipher = EXCLUDED.key_cipher,
    key_hint = EXCLUDED.key_hint
RETURNING id, user_id, org_id, provider, key_cipher, key_hint, created_at;

-- name: GetAIProviderKeyForOrg :one
SELECT id, user_id, org_id, provider, key_cipher, key_hint, created_at
FROM ai_provider_keys
WHERE org_id = $1 AND provider = $2;

-- name: GetAIProviderKeyForUser :one
SELECT id, user_id, org_id, provider, key_cipher, key_hint, created_at
FROM ai_provider_keys
WHERE user_id = $1 AND provider = $2;

-- name: ListAIProviderKeysByOrg :many
SELECT id, user_id, org_id, provider, key_hint, created_at
FROM ai_provider_keys
WHERE org_id = $1
ORDER BY provider ASC;

-- name: ListAIProviderKeysByUser :many
SELECT id, user_id, org_id, provider, key_hint, created_at
FROM ai_provider_keys
WHERE user_id = $1
ORDER BY provider ASC;

-- name: DeleteAIProviderKeyForOrg :exec
DELETE FROM ai_provider_keys WHERE org_id = $1 AND provider = $2;

-- name: DeleteAIProviderKeyForUser :exec
DELETE FROM ai_provider_keys WHERE user_id = $1 AND provider = $2;
