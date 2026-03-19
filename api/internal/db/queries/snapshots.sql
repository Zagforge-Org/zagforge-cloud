-- name: InsertSnapshot :one
INSERT INTO snapshots (repo_id, job_id, branch, commit_sha, gcs_path, snapshot_version, zigzag_version, size_bytes)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetLatestSnapshot :one
SELECT * FROM snapshots
WHERE repo_id = $1 AND branch = $2
ORDER BY created_at DESC
LIMIT 1;

-- name: GetSnapshotsByBranch :many
SELECT * FROM snapshots
WHERE repo_id = $1 AND branch = $2
ORDER BY created_at DESC;

-- name: GetSnapshotByID :one
SELECT * FROM snapshots WHERE id = $1;
