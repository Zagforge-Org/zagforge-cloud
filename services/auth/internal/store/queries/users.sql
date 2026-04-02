-- name: CreateUser :one
INSERT INTO users (email, email_verified, first_name, last_name, avatar_url)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: UpdateUserProfile :one
UPDATE users
SET first_name = $2,
    last_name = $3,
    nickname = $4,
    bio = $5,
    country = $6,
    age = $7,
    timezone = $8,
    language = $9,
    social_links = $10,
    profile_visibility = $11
WHERE id = $1
RETURNING *;

-- name: UpdateUserAvatar :exec
UPDATE users SET avatar_url = $2 WHERE id = $1;

-- name: UpdateUserOnboardingStep :exec
UPDATE users SET onboarding_step = $2 WHERE id = $1;

-- name: UpdateUserPhone :exec
UPDATE users SET phone_cipher = $2 WHERE id = $1;

-- name: ListUsers :many
SELECT * FROM users
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountUsers :one
SELECT count(*) FROM users;

-- name: UpdateUserPlatformAdmin :exec
UPDATE users SET is_platform_admin = $2 WHERE id = $1;

-- name: DeleteUser :exec
DELETE FROM users WHERE id = $1;
