-- name: InsertContextToken :one
INSERT INTO context_tokens (repo_id, user_id, org_id,
target_snapshot_id, token_hash, label, expires_at, visibility)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, repo_id, user_id, org_id, target_snapshot_id,
token_hash, label, last_used_at, expires_at, visibility, created_at;

-- name: GetContextTokenByHash :one
SELECT id, repo_id, user_id, org_id, target_snapshot_id, token_hash,
    label, last_used_at, expires_at, visibility, created_at
FROM context_tokens
WHERE token_hash = $1;

-- name: ListContextTokensByRepo :many
SELECT ct.id, ct.repo_id, ct.user_id, ct.org_id, ct.target_snapshot_id,
    ct.label, ct.last_used_at, ct.expires_at, ct.visibility, ct.created_at,
    (SELECT count(*) FROM context_token_allowed_users WHERE token_id = ct.id)::int AS allowed_user_count
FROM context_tokens ct
WHERE ct.repo_id = $1
ORDER BY ct.created_at DESC;

-- name: UpdateContextTokenLastUsed :exec
UPDATE context_tokens SET last_used_at = now() WHERE id = $1;

-- name: DeleteContextTokenForOrg :exec
DELETE FROM context_tokens WHERE id = $1 AND org_id = $2;

-- name: DeleteContextTokenForUser :exec
DELETE FROM context_tokens WHERE id = $1 AND user_id = $2;

-- name: DeleteExpiredContextTokens :exec
DELETE FROM context_tokens WHERE expires_at IS NOT NULL AND expires_at < now();
