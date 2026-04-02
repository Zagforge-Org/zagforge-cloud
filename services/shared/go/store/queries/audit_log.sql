-- name: InsertAuditLog :one
INSERT INTO audit_log (user_id, org_id, actor_id, action, target_id, metadata)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListAuditLogByOrg :many
SELECT a.*, u.username AS actor_username
FROM audit_log a
JOIN users u ON u.id = a.actor_id
WHERE a.org_id = $1
  AND a.created_at < $2
ORDER BY a.created_at DESC
LIMIT $3;

-- name: ListAuditLogByUser :many
SELECT a.*, u.username AS actor_username
FROM audit_log a
JOIN users u ON u.id = a.actor_id
WHERE a.user_id = $1
  AND a.created_at < $2
ORDER BY a.created_at DESC
LIMIT $3;
