-- name: CreateOrganization :one
INSERT INTO organizations (slug, name)
VALUES ($1, $2)
RETURNING *;

-- name: GetOrganizationByID :one
SELECT * FROM organizations WHERE id = $1;

-- name: GetOrganizationBySlug :one
SELECT * FROM organizations WHERE slug = $1;

-- name: UpdateOrganization :one
UPDATE organizations
SET name = $2, logo_url = $3, billing_email = $4
WHERE id = $1
RETURNING *;

-- name: DeleteOrganization :exec
DELETE FROM organizations WHERE id = $1;

-- name: ListUserOrganizations :many
SELECT o.* FROM organizations o
JOIN org_memberships om ON o.id = om.org_id
WHERE om.user_id = $1
ORDER BY o.name;

-- name: CreateOrgMembership :one
INSERT INTO org_memberships (org_id, user_id, role)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetOrgMembership :one
SELECT * FROM org_memberships
WHERE org_id = $1 AND user_id = $2;

-- name: ListOrgMembers :many
SELECT om.*, u.email, u.first_name, u.last_name, u.avatar_url
FROM org_memberships om
JOIN users u ON om.user_id = u.id
WHERE om.org_id = $1
ORDER BY om.role, u.email;

-- name: UpdateOrgMemberRole :exec
UPDATE org_memberships SET role = $3
WHERE org_id = $1 AND user_id = $2;

-- name: DeleteOrgMembership :exec
DELETE FROM org_memberships WHERE org_id = $1 AND user_id = $2;

-- name: CountOrgMembers :one
SELECT count(*) FROM org_memberships WHERE org_id = $1;

-- name: ListOrganizations :many
SELECT * FROM organizations
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountOrganizations :one
SELECT count(*) FROM organizations;

-- name: UpdateOrganizationPlan :one
UPDATE organizations
SET plan = $2, max_members = $3
WHERE id = $1
RETURNING *;
