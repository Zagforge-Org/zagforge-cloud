-- name: CreateTeam :one
INSERT INTO teams (org_id, slug, name, description)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetTeamByID :one
SELECT * FROM teams WHERE id = $1;

-- name: ListTeamsByOrg :many
SELECT * FROM teams
WHERE org_id = $1
ORDER BY name;

-- name: UpdateTeam :one
UPDATE teams
SET name = $2, slug = $3, description = $4
WHERE id = $1
RETURNING *;

-- name: DeleteTeam :exec
DELETE FROM teams WHERE id = $1;

-- name: CreateTeamMembership :one
INSERT INTO team_memberships (team_id, user_id, role)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetTeamMembership :one
SELECT * FROM team_memberships
WHERE team_id = $1 AND user_id = $2;

-- name: ListTeamMembers :many
SELECT tm.*, u.email, u.first_name, u.last_name, u.avatar_url
FROM team_memberships tm
JOIN users u ON tm.user_id = u.id
WHERE tm.team_id = $1
ORDER BY tm.role, u.email;

-- name: UpdateTeamMemberRole :exec
UPDATE team_memberships SET role = $3
WHERE team_id = $1 AND user_id = $2;

-- name: DeleteTeamMembership :exec
DELETE FROM team_memberships WHERE team_id = $1 AND user_id = $2;

-- name: CountTeamMembers :one
SELECT count(*) FROM team_memberships WHERE team_id = $1;

-- name: ListUserTeams :many
SELECT t.* FROM teams t
JOIN team_memberships tm ON t.id = tm.team_id
WHERE tm.user_id = $1 AND t.org_id = $2
ORDER BY t.name;
