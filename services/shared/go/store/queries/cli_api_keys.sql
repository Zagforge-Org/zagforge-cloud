-- name: InsertCLIAPIKey :one
INSERT INTO cli_api_keys (org_id, user_id, key_hash, key_hint, label)
VALUES (@org_id, @user_id, @key_hash, @key_hint, @label)
RETURNING *;

-- name: GetCLIAPIKeyByHash :one
SELECT * FROM cli_api_keys WHERE key_hash = @key_hash;

-- name: ListCLIAPIKeysByOrg :many
SELECT id, org_id, user_id, key_hint, label, created_at
FROM cli_api_keys
WHERE org_id = @org_id
ORDER BY created_at DESC;

-- name: ListCLIAPIKeysByUser :many
SELECT id, org_id, user_id, key_hint, label, created_at
FROM cli_api_keys
WHERE user_id = @user_id
ORDER BY created_at DESC;

-- name: DeleteCLIAPIKeyForOrg :exec
DELETE FROM cli_api_keys WHERE id = @id AND org_id = @org_id;

-- name: DeleteCLIAPIKeyForUser :exec
DELETE FROM cli_api_keys WHERE id = @id AND user_id = @user_id;
