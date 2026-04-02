-- name: CreateInvite :one
INSERT INTO invites (org_id, team_id, invited_by, email, role, token_hash, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetInviteByID :one
SELECT * FROM invites WHERE id = $1;

-- name: GetInviteByTokenHash :one
SELECT * FROM invites
WHERE token_hash = $1 AND status = 'pending' AND expires_at > now();

-- name: ListOrgInvites :many
SELECT * FROM invites
WHERE org_id = $1
ORDER BY created_at DESC;

-- name: AcceptInvite :one
UPDATE invites
SET status = 'accepted', accepted_at = now()
WHERE id = $1 AND status = 'pending' AND expires_at > now()
RETURNING *;

-- name: RevokeInvite :exec
UPDATE invites SET status = 'revoked'
WHERE id = $1 AND org_id = $2 AND status = 'pending';

-- name: DeleteExpiredInvites :exec
UPDATE invites SET status = 'expired'
WHERE status = 'pending' AND expires_at < now();
