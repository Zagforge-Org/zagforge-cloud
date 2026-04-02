-- name: CreateSession :one
INSERT INTO sessions (user_id, ip_address, user_agent, device_name, device_fingerprint, country, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetSessionByID :one
SELECT * FROM sessions WHERE id = $1;

-- name: ListActiveSessions :many
SELECT * FROM sessions
WHERE user_id = $1 AND revoked_at IS NULL AND expires_at > now()
ORDER BY last_active_at DESC;

-- name: UpdateSessionLastActive :exec
UPDATE sessions SET last_active_at = now() WHERE id = $1;

-- name: RevokeSession :exec
UPDATE sessions SET revoked_at = now() WHERE id = $1;

-- name: RevokeAllUserSessions :exec
UPDATE sessions SET revoked_at = now()
WHERE user_id = $1 AND revoked_at IS NULL;

-- name: DeleteExpiredSessions :exec
DELETE FROM sessions WHERE expires_at < now() - interval '7 days';
