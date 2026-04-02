ALTER TABLE organizations RENAME COLUMN auth_org_id TO zitadel_org_id;

ALTER TABLE users RENAME COLUMN auth_user_id TO zitadel_user_id;

ALTER TABLE sessions RENAME COLUMN auth_session_id TO zitadel_session_id;
