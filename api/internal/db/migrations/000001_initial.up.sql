CREATE TABLE organizations (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    clerk_org_id TEXT UNIQUE NOT NULL,
    slug         TEXT UNIQUE NOT NULL,
    name         TEXT NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE repositories (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES organizations(id),
    github_repo_id  BIGINT UNIQUE NOT NULL,
    installation_id BIGINT NOT NULL,
    full_name       TEXT NOT NULL,
    default_branch  TEXT NOT NULL DEFAULT 'main',
    installed_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_repositories_org_id ON repositories (org_id);
CREATE INDEX idx_repositories_full_name ON repositories (full_name);

CREATE TABLE jobs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    repo_id         UUID NOT NULL REFERENCES repositories(id),
    branch          TEXT NOT NULL,
    commit_sha      TEXT NOT NULL,
    delivery_id     TEXT,
    status          TEXT NOT NULL DEFAULT 'queued'
                    CHECK (status IN ('queued', 'running', 'succeeded', 'failed', 'cancelled', 'superseded')),
    error_message   TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at      TIMESTAMPTZ,
    finished_at     TIMESTAMPTZ
);

CREATE INDEX idx_jobs_active_branch ON jobs (repo_id, branch)
    WHERE status IN ('queued', 'running');

CREATE UNIQUE INDEX idx_jobs_delivery_id ON jobs (delivery_id)
    WHERE delivery_id IS NOT NULL;

CREATE INDEX idx_jobs_running ON jobs (status, started_at)
    WHERE status = 'running';

CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER jobs_set_updated_at
BEFORE UPDATE ON jobs
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE snapshots (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    repo_id          UUID NOT NULL REFERENCES repositories(id),
    job_id           UUID NOT NULL REFERENCES jobs(id),
    branch           TEXT NOT NULL,
    commit_sha       TEXT NOT NULL,
    gcs_path         TEXT NOT NULL,
    snapshot_version INT NOT NULL DEFAULT 1,
    zigzag_version   TEXT NOT NULL,
    size_bytes       BIGINT NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_snapshots_latest ON snapshots (repo_id, branch, created_at DESC);
CREATE INDEX idx_snapshots_job_id ON snapshots (job_id);
CREATE UNIQUE INDEX idx_snapshots_unique ON snapshots (repo_id, branch, commit_sha);
