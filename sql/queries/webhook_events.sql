-- name: CreateWebhookEvent :one
INSERT INTO webhook_events (event_id, event_type, payload)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetWebhookEventByEventID :one
SELECT * FROM webhook_events WHERE event_id = $1;

-- name: MarkWebhookProcessed :exec
UPDATE webhook_events
SET processed = TRUE, processed_at = NOW()
WHERE id = $1;

-- name: MarkWebhookError :exec
UPDATE webhook_events
SET error = $2
WHERE id = $1;

-- name: ListUnprocessedWebhookEvents :many
SELECT * FROM webhook_events
WHERE NOT processed
ORDER BY created_at ASC
LIMIT $1;
