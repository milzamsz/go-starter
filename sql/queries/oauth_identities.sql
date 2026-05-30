-- name: CreateOAuthIdentity :one
INSERT INTO oauth_identities (user_id, provider, provider_user_id, access_token, refresh_token, expires_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetOAuthIdentity :one
SELECT * FROM oauth_identities
WHERE provider = $1 AND provider_user_id = $2;

-- name: GetOAuthIdentitiesByUser :many
SELECT * FROM oauth_identities
WHERE user_id = $1
ORDER BY created_at DESC;

-- name: UpdateOAuthTokens :exec
UPDATE oauth_identities
SET access_token = $2, refresh_token = $3, expires_at = $4, updated_at = NOW()
WHERE id = $1;

-- name: DeleteOAuthIdentity :exec
DELETE FROM oauth_identities WHERE id = $1;
