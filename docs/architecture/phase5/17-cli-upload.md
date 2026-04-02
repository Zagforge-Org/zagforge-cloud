# Zagforge — CLI Upload (`zigzag --upload`) [Phase 5]

The `--upload` flag is the first ingestion mechanism for Phase 5. It lets any developer push a snapshot from their local machine to Zagforge — regardless of whether their repo is on GitHub, whether they have the GitHub App installed, or whether they want webhook automation.

This is the Open Core play: the flag lives in the open source Zigzag repo where users can read exactly what data leaves their machine.

---

## Open Source Placement

- **Repo:** main Zigzag CLI repo (open source) — written in Zig
- **File:** `src/cli/handlers/upload.zig` — all upload logic is isolated here
- **Coupling:** zero. The core Zigzag engine is unchanged. `--upload` is a standard flag handler, identical in shape to every other flag (e.g. `--json`, `--llm-report`).

**Step 1 — add `upload` field to `Config` in `src/cli/commands/config/config.zig`:**

```zig
pub const Config = struct {
    // ... existing fields ...
    upload: bool, // Upload snapshot to Zagforge cloud
};
```

**Step 2 — register the flag in `src/cli/flags.zig`:**

```zig
const uploadHandler = @import("./handlers/upload.zig").handleUpload;

pub const flags = [_]FlagsHandler{
    // ... existing flags ...
    .{ .name = "--upload", .takes_value = false, .handler = &uploadHandler },
};
```

**Step 3 — implement `src/cli/handlers/upload.zig`:**

```zig
const std = @import("std");
const Config = @import("../commands/config/config.zig").Config;

pub fn handleUpload(cfg: *Config, allocator: std.mem.Allocator, _: ?[]const u8) anyerror!void {
    _ = allocator;
    cfg.upload = true;
}
```

The actual HTTP upload call happens after the engine finishes, in the runner — `cfg.upload` is checked post-run the same way `cfg.json_output` triggers JSON report emission.

---

## What Is Sent

The runner POSTs a `snapshot_version: 2` JSON payload — **no file contents**:

```json
{
  "org_slug": "zagforge",
  "repo_full_name": "zagforge/zigzag",
  "commit_sha": "3fa912e1abc456def789012345678901abcdef01",
  "branch": "main",
  "metadata_snapshot": {
    "snapshot_version": 2,
    "zigzag_version": "0.12.0",
    "commit_sha": "3fa912e1...",
    "branch": "main",
    "generated_at": "2026-03-21T12:00:00Z",
    "summary": { "source_files": 42, "total_lines": 8500 },
    "file_tree": [
      { "path": "cmd/api/main.go", "language": "go", "lines": 87, "sha": "abc123" }
    ]
  }
}
```

`file_tree[].sha` is the Git blob SHA for each file, used by the Context Proxy to fetch individual file contents from GitHub at query time.

**No source code ever leaves the developer's machine via `--upload`.** The open source placement of this logic is intentional: developers can audit exactly what is sent.

---

## Auth: CLI Tokens (`zf_pk_*`)

CLI tokens are long-lived API keys issued by Zagforge, distinct from Zitadel OIDC session JWTs.

**Format:** `zf_pk_<random>` — prefix `zf_pk_` enables GitHub secret scanning (automatic alerts if committed).

**Storage:** Only the SHA-256 hash is stored server-side (in `api_keys` table or equivalent). The raw token is shown once at creation.

**Middleware:** `POST /api/v1/upload` uses a dedicated CLI token middleware, not the OIDC JWT verification middleware. The middleware:
1. Extracts the bearer token from `Authorization` header
2. Computes SHA-256 hash, looks up in token store
3. Resolves to `org_id`
4. Sets org context for the handler

---

## Upload Endpoint: Backend Logic

```
POST /api/v1/upload

1. Verify CLI bearer token (dedicated middleware)
2. Resolve org_slug → org_id; assert token is scoped to that org
3. Look up repositories by repo_full_name + org_id:
   → Not found, no GitHub App installation:
       404 { "error": "repo_not_connected",
             "message": "Install the Zagforge GitHub App, or ensure your org slug is correct." }
   → Not found but installation_id is known (GitHub App installed, no prior upload):
       Auto-create repositories row
4. Insert snapshot row:
   → job_id = NULL (CLI upload, no worker job)
   → snapshot_version = 2
   → metadata = { ignore_patterns, zigzag_config_hash } from payload
5. Write snapshot.json to GCS at {org_uuid}/{repo_uuid}/{commit_sha}/snapshot.json

Response: 201 { "snapshot_id": "<uuid>", "created_at": "<timestamp>" }
```

---

## CLI UX

```bash
# First-time setup
export ZAGFORGE_API_KEY=zf_pk_...

# Upload
cd my-project
zigzag --upload

# Output on success
✓ Snapshot uploaded (42 files, 8500 lines)
  View at: https://cloud.zagforge.com/repos/zagforge/my-project
  Context URL: https://api.zagforge.com/v1/context/zf_ctx_...
```

The CLI prints the Context URL on success so the developer can paste it immediately into their AI tool.

---

## `ZAGFORGE_API_KEY` Discovery

The CLI looks for the key in this order:
1. `ZAGFORGE_API_KEY` environment variable
2. `~/.zagforge/credentials` config file (written by `zigzag login`, future Phase)
3. Missing → prints setup instructions and exits 1

---

## GitHub Webhook Automation (Phase 5b)

The `--upload` flag is Phase 5. Automating uploads via the existing GitHub webhook (push event → worker → snapshot) is Phase 5b. The data model and GCS layout are identical between both paths — the only difference is `job_id` (NULL for CLI, set for webhook).

When Phase 5b ships:
- Users who have the GitHub App installed get automatic snapshots on every push
- `--upload` remains available for local-only repos, CI overrides, and manual triggers
- The two paths are additive, not mutually exclusive
