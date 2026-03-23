-- name: UpsertOrg :one
INSERT INTO organizations (clerk_org_id, slug, name)
VALUES ($1, $2, $3)
ON CONFLICT (clerk_org_id) DO UPDATE
    SET name = EXCLUDED.name
RETURNING *;

-- name: GetOrgByClerkID :one
SELECT * FROM organizations WHERE clerk_org_id = $1;

-- name: GetOrganizationBySlug :one
SELECT * FROM organizations WHERE slug = $1;
