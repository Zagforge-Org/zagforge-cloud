-- name: CreateMembership :one
INSERT INTO memberships (user_id, org_id, role, invited_by)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetMembership :one
SELECT * FROM memberships WHERE user_id = $1 AND org_id = $2;

-- name: ListMembershipsByUser :many
SELECT m.*, o.slug AS org_slug, o.name AS org_name
FROM memberships m
JOIN organizations o ON o.id = m.org_id
WHERE m.user_id = $1
ORDER BY m.joined_at ASC;

-- name: ListMembershipsByOrg :many
SELECT m.*, u.username, u.email, u.avatar_url
FROM memberships m
JOIN users u ON u.id = m.user_id
WHERE m.org_id = $1
ORDER BY m.joined_at ASC;

-- name: UpdateMembershipRole :one
UPDATE memberships SET role = $3
WHERE user_id = $1 AND org_id = $2
RETURNING *;

-- name: DeleteMembership :exec
DELETE FROM memberships WHERE user_id = $1 AND org_id = $2;

-- name: CountMembershipsByOrg :one
SELECT count(*) FROM memberships WHERE org_id = $1;

-- name: CountOwnersByOrg :one
SELECT count(*) FROM memberships WHERE org_id = $1 AND role = 'owner';
