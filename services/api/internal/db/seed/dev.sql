-- Dev seed data. Idempotent — safe to run multiple times.
-- Override GITHUB_REPO_ID and INSTALLATION_ID via psql variables if needed:
--   psql -v github_repo_id=12345 -v installation_id=67890 -f dev.sql

INSERT INTO organizations (auth_org_id, slug, name)
VALUES ('dev_org', 'legationpro', 'LegationPro')
ON CONFLICT (slug) DO NOTHING;

INSERT INTO repositories (org_id, github_repo_id, installation_id, full_name, default_branch)
SELECT o.id, 1185153652, 117271770, 'LegationPro/zagforge-mvp', 'main'
FROM organizations o WHERE o.slug = 'legationpro'
ON CONFLICT (github_repo_id) DO NOTHING;
