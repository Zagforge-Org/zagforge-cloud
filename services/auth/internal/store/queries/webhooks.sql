-- name: CreateWebhookSubscription :one
INSERT INTO webhook_subscriptions (org_id, url, secret, events)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetWebhookSubscription :one
SELECT * FROM webhook_subscriptions WHERE id = $1;

-- name: ListWebhookSubscriptions :many
SELECT * FROM webhook_subscriptions
WHERE org_id = $1
ORDER BY created_at DESC;

-- name: UpdateWebhookSubscription :one
UPDATE webhook_subscriptions
SET url = $2, events = $3, active = $4
WHERE id = $1
RETURNING *;

-- name: DeleteWebhookSubscription :exec
DELETE FROM webhook_subscriptions WHERE id = $1;

-- name: ListActiveWebhooksByEvent :many
SELECT * FROM webhook_subscriptions
WHERE org_id = $1 AND active = true AND $2 = ANY(events);

-- name: CreateWebhookDelivery :one
INSERT INTO webhook_deliveries (subscription_id, event, payload, attempts, next_retry_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListWebhookDeliveries :many
SELECT * FROM webhook_deliveries
WHERE subscription_id = $1
ORDER BY created_at DESC
LIMIT $2;

-- name: UpdateWebhookDeliveryResult :exec
UPDATE webhook_deliveries
SET response_status = $2, response_body = $3, delivered_at = $4, attempts = $5, next_retry_at = $6
WHERE id = $1;

-- name: ListPendingDeliveries :many
SELECT * FROM webhook_deliveries
WHERE delivered_at IS NULL AND (next_retry_at IS NULL OR next_retry_at <= now())
ORDER BY created_at ASC
LIMIT $1;
