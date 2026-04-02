-- name: CreateOAuthState :exec
INSERT INTO oauth_states (state, provider, redirect_uri, expires_at)
VALUES ($1, $2, $3, $4);

-- name: GetAndDeleteOAuthState :one
DELETE FROM oauth_states
WHERE state = $1 AND expires_at > now()
RETURNING *;

-- name: DeleteExpiredOAuthStates :exec
DELETE FROM oauth_states WHERE expires_at < now();
