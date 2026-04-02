-- name: CreateOAuthIdentity :one
INSERT INTO oauth_identities (user_id, provider, provider_id, email, display_name, avatar_url, access_token, refresh_token, token_expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: GetOAuthIdentity :one
SELECT * FROM oauth_identities
WHERE provider = $1 AND provider_id = $2;

-- name: ListOAuthIdentitiesByUser :many
SELECT * FROM oauth_identities WHERE user_id = $1 ORDER BY provider;

-- name: UpdateOAuthTokens :exec
UPDATE oauth_identities
SET access_token = $3, refresh_token = $4, token_expires_at = $5
WHERE provider = $1 AND provider_id = $2;

-- name: DeleteOAuthIdentity :exec
DELETE FROM oauth_identities WHERE user_id = $1 AND provider = $2;

-- name: CountOAuthIdentitiesByUser :one
SELECT count(*) FROM oauth_identities WHERE user_id = $1;
