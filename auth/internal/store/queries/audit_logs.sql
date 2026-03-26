-- name: CreateAuditLog :exec
INSERT INTO audit_logs (org_id, actor_id, action, target_type, target_id, ip_address, user_agent, metadata)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: ListAuditLogs :many
SELECT * FROM audit_logs
WHERE org_id = $1 AND created_at < $2
ORDER BY created_at DESC
LIMIT $3;

-- name: ListAuditLogsByAction :many
SELECT * FROM audit_logs
WHERE org_id = $1 AND action = $2 AND created_at < $3
ORDER BY created_at DESC
LIMIT $4;

-- name: ListAuditLogsByDateRange :many
SELECT * FROM audit_logs
WHERE org_id = $1 AND created_at >= $2 AND created_at <= $3
ORDER BY created_at DESC
LIMIT $4;

-- name: CountLoginsByDay :many
SELECT date_trunc('day', created_at)::date AS day, count(*) AS total
FROM audit_logs
WHERE org_id = $1 AND action = 'user.login' AND created_at >= $2 AND created_at <= $3
GROUP BY day
ORDER BY day DESC;

-- name: CountFailedLoginsByDay :many
SELECT date_trunc('day', created_at)::date AS day, count(*) AS total
FROM failed_login_attempts
WHERE created_at >= $1 AND created_at <= $2
GROUP BY day
ORDER BY day DESC;
