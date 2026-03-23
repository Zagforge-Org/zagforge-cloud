-- name: InsertContextToken :one
INSERT INTO context_tokens (repo_id, org_id,
target_snapshot_id, token_hash, label, expires_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, repo_id, org_id, target_snapshot_id,
token_hash, label, last_used_at, expires_at, created_at;

-- name: GetContextTokenByHash :one
SELECT id, repo_id, org_id, target_snapshot_id, token_hash,
    label, last_used_at, expires_at, created_at
FROM context_tokens
WHERE token_hash = $1;

-- name: ListContextTokensByRepo :many
SELECT id, repo_id, org_id, target_snapshot_id, label,
    last_used_at, expires_at, created_at
FROM context_tokens
WHERE repo_id = $1
ORDER BY created_at DESC;

-- name: UpdateContextTokenLastUsed :exec
UPDATE context_tokens SET last_used_at = now() WHERE id = $1;

-- name: DeleteContextToken :exec
DELETE FROM context_tokens WHERE id = $1 AND org_id = $2;

-- name: DeleteExpiredContextTokens :exec
DELETE FROM context_tokens WHERE expires_at IS NOT NULL AND expires_at < now();
