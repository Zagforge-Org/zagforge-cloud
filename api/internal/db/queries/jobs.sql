-- name: CreateJob :one
INSERT INTO jobs (repo_id, branch, commit_sha, delivery_id)
VALUES ($1, $2, $3, NULLIF($4, ''))
RETURNING *;

-- name: GetActiveJobsForBranch :many
SELECT * FROM jobs
WHERE repo_id = $1
  AND branch = $2
  AND status IN ('queued', 'running')
ORDER BY created_at ASC;

-- name: MarkJobSuperseded :exec
UPDATE jobs
SET status = 'superseded'
WHERE id = $1
  AND status = 'queued';

-- name: UpdateJobStatus :exec
UPDATE jobs
SET status = $2,
    error_message = $3,
    started_at    = CASE WHEN $2 = 'running' THEN now() ELSE started_at END,
    finished_at   = CASE WHEN $2 IN ('succeeded', 'failed', 'cancelled', 'superseded') THEN now() ELSE finished_at END
WHERE id = $1;
