-- Migration: Rename Zitadel-specific columns to generic auth columns.
-- The auth service is now fully custom (no Zitadel), so column names
-- should reflect the actual auth service identity.

ALTER TABLE organizations RENAME COLUMN zitadel_org_id TO auth_org_id;

ALTER TABLE users RENAME COLUMN zitadel_user_id TO auth_user_id;

ALTER TABLE sessions RENAME COLUMN zitadel_session_id TO auth_session_id;
