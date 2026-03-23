-- name: UpsertRepo :one
INSERT INTO repositories (org_id, github_repo_id, installation_id, full_name, default_branch)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (github_repo_id) DO UPDATE
    SET installation_id = EXCLUDED.installation_id,
        full_name       = EXCLUDED.full_name,
        default_branch  = EXCLUDED.default_branch
RETURNING *;

-- name: GetRepoByGithubID :one
SELECT * FROM repositories WHERE github_repo_id = $1;

-- name: GetRepoByID :one
SELECT * FROM repositories WHERE id = $1;

-- name: ListReposByOrg :many
SELECT * FROM repositories
WHERE org_id = $1
  AND full_name > $2
ORDER BY full_name ASC
LIMIT $3;

-- name: GetRepoByFullNameAndOrg :one
SELECT * FROM repositories WHERE full_name = $1 AND org_id = $2;
