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

-- name: InsertCLISnapshot :one
INSERT INTO snapshots (repo_id, branch, commit_sha, gcs_path,
    snapshot_version, zigzag_version, size_bytes, metadata
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (repo_id, branch, commit_sha) DO UPDATE
 SET gcs_path = EXCLUDED.gcs_path,
     snapshot_version = EXCLUDED.snapshot_version,
     zigzag_version = EXCLUDED.zigzag_version,
     size_bytes = EXCLUDED.size_bytes,
     metadata = EXCLUDED.metadata
RETURNING id, repo_id, job_id, branch, commit_sha, gcs_path,
    snapshot_version, zigzag_version, size_bytes, metadata,
    created_at;
