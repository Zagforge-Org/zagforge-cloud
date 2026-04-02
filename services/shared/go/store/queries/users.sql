-- name: UpsertUser :one
INSERT INTO users (auth_user_id, username, email, email_verified, phone, avatar_url)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (auth_user_id) DO UPDATE
    SET username       = EXCLUDED.username,
        email          = EXCLUDED.email,
        email_verified = EXCLUDED.email_verified,
        phone          = EXCLUDED.phone,
        avatar_url     = EXCLUDED.avatar_url
RETURNING *;

-- name: GetUserByAuthID :one
SELECT * FROM users WHERE auth_user_id = $1;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByUsername :one
SELECT * FROM users WHERE username = $1;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: UpdateUser :one
UPDATE users
SET username       = COALESCE(NULLIF(sqlc.arg(username)::text, ''), username),
    email          = COALESCE(NULLIF(sqlc.arg(email)::text, ''), email),
    email_verified = sqlc.arg(email_verified),
    phone          = sqlc.arg(phone),
    avatar_url     = sqlc.arg(avatar_url)
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: DeleteUser :exec
DELETE FROM users WHERE id = $1;
