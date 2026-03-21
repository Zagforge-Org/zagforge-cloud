# Zagforge — Storage & Snapshot Format [Phase 2]

## GCS Object Layout

Uses full UUIDs for org and repo, and full 40-character commit SHAs:

```
gs://zagforge-snapshots/
  {org_uuid}/
    {repo_uuid}/
      {full_commit_sha}/
        snapshot.json
```

Example:
```
gs://zagforge-snapshots/a1b2c3d4-e5f6-7890-abcd-ef1234567890/f9e8d7c6-b5a4-3210-fedc-ba0987654321/3fa912e1abc456def789012345678901abcdef01/snapshot.json
```

**IAM roles:**
- Worker container service account: `roles/storage.objectCreator` on the bucket (write-only)
- API service account: `roles/storage.objectViewer` on the bucket (read-only)

---

## Snapshot Format

```json
{
  "snapshot_version": 1,
  "zigzag_version": "0.11.0",
  "commit_sha": "3fa912e1abc456def789012345678901abcdef01",
  "branch": "main",
  "generated_at": "2026-03-14T12:00:00Z",
  "summary": {
    "source_files": 42,
    "total_lines": 8500,
    "total_size_bytes": 245000,
    "languages": [
      { "name": "go", "files": 30, "lines": 6200 },
      { "name": "sql", "files": 12, "lines": 2300 }
    ]
  },
  "files": [
    {
      "path": "cmd/api/main.go",
      "language": "go",
      "lines": 87,
      "content": "package main\n..."
    }
  ]
}
```

The `snapshot_version` field allows the API to handle format migrations as Zigzag evolves.

---

## Phase 5: Snapshot Version 2

Phase 5 changes the snapshot format to remove file contents. Code never rests in Zagforge storage — the backend fetches it from GitHub at query time.

**`files[]` is replaced by `file_tree[]`:**

```json
{
  "snapshot_version": 2,
  "zigzag_version": "0.12.0",
  "commit_sha": "3fa912e1abc456def789012345678901abcdef01",
  "branch": "main",
  "generated_at": "2026-03-21T12:00:00Z",
  "summary": {
    "source_files": 42,
    "total_lines": 8500,
    "total_size_bytes": 245000,
    "languages": [
      { "name": "go", "files": 30, "lines": 6200 },
      { "name": "sql", "files": 12, "lines": 2300 }
    ]
  },
  "file_tree": [
    { "path": "cmd/api/main.go", "language": "go", "lines": 87, "sha": "abc123" }
  ]
}
```

`file_tree[].sha` is the Git blob SHA for each file, used by the Context Proxy to fetch specific file versions from GitHub via the installation token.

**Backward compatibility:** When the Context Proxy encounters a v1 snapshot, it returns `422 { "error": "snapshot_outdated", "message": "Re-run zigzag --upload to generate a v2 snapshot." }`. V1 snapshots remain readable via the existing public read endpoints (`/api/v1/{org}/{repo}/latest`, etc.) without change.

**GCS path:** unchanged — `{org_uuid}/{repo_uuid}/{commit_sha}/snapshot.json`.

**IAM:** unchanged — API service account retains `roles/storage.objectViewer`; CLI upload goes through the Go API (which writes to GCS), not directly from the CLI.

## Context Assembly Cache (Phase 5)

The assembled `report.llm.md` (file contents fetched from GitHub and stitched together) is cached in Redis:

```
Key:   ctx:{repo_id}:{commit_sha}
Value: fully assembled markdown string
TTL:   10 minutes
```

This is an in-memory cache only. The assembled markdown is never written to GCS or Postgres. Redis namespace `ctx:` is separate from rate limiter keys (`rl:`).
