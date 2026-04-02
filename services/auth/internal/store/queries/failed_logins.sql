-- name: RecordFailedLogin :exec
INSERT INTO failed_login_attempts (identifier, ip_address, user_agent)
VALUES ($1, $2, $3);

-- name: CountRecentFailedLogins :one
SELECT count(*) FROM failed_login_attempts
WHERE identifier = $1 AND created_at > now() - interval '15 minutes';

-- name: CountRecentFailedLoginsByIP :one
SELECT count(*) FROM failed_login_attempts
WHERE ip_address = $1 AND created_at > now() - interval '15 minutes';

-- name: DeleteOldFailedLogins :exec
DELETE FROM failed_login_attempts WHERE created_at < now() - interval '24 hours';
