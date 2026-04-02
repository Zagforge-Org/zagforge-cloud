-- name: UpsertSession :one
INSERT INTO sessions (user_id, auth_session_id, device_name, ip_address)
VALUES ($1, $2, $3, $4)
ON CONFLICT (auth_session_id) DO UPDATE
    SET last_active_at = now(),
        device_name    = EXCLUDED.device_name,
        ip_address     = EXCLUDED.ip_address
RETURNING *;

-- name: ListSessionsByUser :many
SELECT * FROM sessions
WHERE user_id = $1
ORDER BY last_active_at DESC;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE id = $1 AND user_id = $2;

-- name: DeleteSessionByAuthID :exec
DELETE FROM sessions WHERE auth_session_id = $1;

-- name: DeleteAllSessionsByUser :exec
DELETE FROM sessions WHERE user_id = $1;
