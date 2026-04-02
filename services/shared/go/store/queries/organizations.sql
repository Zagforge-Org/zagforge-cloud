-- name: UpsertOrg :one
INSERT INTO organizations (auth_org_id, slug, name)
VALUES ($1, $2, $3)
ON CONFLICT (auth_org_id) DO UPDATE
    SET name = EXCLUDED.name
RETURNING *;

-- name: GetOrgByAuthID :one
SELECT * FROM organizations WHERE auth_org_id = $1;

-- name: GetOrganizationBySlug :one
SELECT * FROM organizations WHERE slug = $1;

-- name: GetOrganizationByID :one
SELECT * FROM organizations WHERE id = $1;

-- name: UpdateOrganization :one
UPDATE organizations SET name = $2, slug = $3
WHERE id = $1
RETURNING *;

-- name: DeleteOrganization :exec
DELETE FROM organizations WHERE id = $1;
