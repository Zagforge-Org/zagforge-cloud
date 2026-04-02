-- name: InsertContextTokenAllowedUser :exec
INSERT INTO context_token_allowed_users (token_id, user_id)
VALUES (@token_id, @user_id)
ON CONFLICT (token_id, user_id) DO NOTHING;

-- name: ListContextTokenAllowedUsers :many
SELECT ctau.id, ctau.user_id, u.username, u.email, ctau.created_at
FROM context_token_allowed_users ctau
JOIN users u ON u.id = ctau.user_id
WHERE ctau.token_id = @token_id
ORDER BY ctau.created_at DESC;

-- name: IsUserAllowedForToken :one
SELECT EXISTS (
    SELECT 1 FROM context_token_allowed_users
    WHERE token_id = @token_id AND user_id = @user_id
) AS allowed;

-- name: ReplaceContextTokenAllowedUsers :exec
DELETE FROM context_token_allowed_users WHERE token_id = @token_id;

-- name: DeleteContextTokenAllowedUser :exec
DELETE FROM context_token_allowed_users
WHERE token_id = @token_id AND user_id = @user_id;
