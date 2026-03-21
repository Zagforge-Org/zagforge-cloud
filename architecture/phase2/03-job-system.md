# Zagforge — Job System [Phase 2]

## Job Status Type

Job status is a typed string constant in Go — not raw string literals scattered through handlers:

```go
type JobStatus string

const (
    JobStatusQueued    JobStatus = "queued"
    JobStatusRunning   JobStatus = "running"
    JobStatusSucceeded JobStatus = "succeeded"
    JobStatusFailed    JobStatus = "failed"
    JobStatusCancelled JobStatus = "cancelled"
)

// Valid returns true if s is a recognized status.
func (s JobStatus) Valid() bool {
    switch s {
    case JobStatusQueued, JobStatusRunning, JobStatusSucceeded, JobStatusFailed, JobStatusCancelled:
        return true
    }
    return false
}

// IsTerminal returns true if the job is in a final state.
func (s JobStatus) IsTerminal() bool {
    return s == JobStatusSucceeded || s == JobStatusFailed || s == JobStatusCancelled
}
```

sqlc maps the `TEXT` column to this type via the `sqlc.yaml` overrides. All handler/engine code uses `JobStatusQueued` etc., never raw `"queued"` strings.

---

## Job Lifecycle (State Machine)

```
queued ──→ running ──→ succeeded
  │           │
  │           └──→ failed
  │
  └──→ cancelled (manual)
```

**Transitions:**

| From | To | Triggered by |
|---|---|---|
| — | `queued` | Webhook received, job created |
| `queued` | `running` | Worker calls `POST /internal/jobs/start` |
| `running` | `succeeded` | Worker callback with snapshot path |
| `running` | `failed` | Worker callback with error, or timeout watchdog |
| `queued` | `cancelled` | Manual cancellation |

**Stale snapshot handling:** When a `running` job completes but a newer `queued` job exists for the same branch, the running job's snapshot is still stored (it's a valid snapshot of that commit). The "latest" API endpoint always serves the most recent snapshot by `created_at`, so the newer job's snapshot will supersede it once processed.

---

## Job Deduplication

The dedup check runs inside a serializable transaction with an advisory lock to prevent race conditions from concurrent webhook deliveries:

```sql
-- Acquire advisory lock scoped to repo_id + branch
SELECT pg_advisory_xact_lock(hashtext($repo_id || ':' || $branch));

-- Check for existing active job
SELECT id, status FROM jobs
WHERE repo_id = $1
  AND branch = $2
  AND status IN ('queued', 'running')
ORDER BY created_at DESC
LIMIT 1;
```

- If a `queued` job exists → update its `commit_sha` to the new commit. **Do not** push a new Cloud Tasks task (the existing task will pick up the updated SHA when the worker reads the job record at execution time).
- If a `running` job exists → create a new `queued` job and push a new Cloud Tasks task.
- If no active job → create a new `queued` job and push a new Cloud Tasks task.

This collapses rapid push sequences (commit A, B, C, D) into a single snapshot of the latest commit.

---

## Job Timeout Watchdog

A Cloud Scheduler cron runs every 5 minutes, hitting an internal API endpoint.

**Auth:** Cloud Scheduler is configured to attach an OIDC token (service account identity). The API validates the token's audience and issuer before processing.

```sql
UPDATE jobs
SET status = 'failed',
    error_message = 'Job timed out',
    finished_at = now()
WHERE status = 'running'
  AND started_at < now() - INTERVAL '20 minutes';
```

The Cloud Run Job's own execution timeout is set to **15 minutes** — shorter than the watchdog's 20-minute window. This ensures the container is killed by Cloud Run before the watchdog fires, giving the worker a chance to report failure via callback. The watchdog is a safety net for cases where the container exits without calling back (OOM kill, infrastructure failure).

---

## Cloud Tasks Configuration

- Max attempts: 3
- Min backoff: 10s
- Max backoff: 300s
- Max doublings: 4
- Task timeout: 20 minutes (aligned with watchdog)
