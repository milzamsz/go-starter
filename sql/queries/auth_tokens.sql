-- name: CreateAuthToken :one
INSERT INTO auth_tokens (user_id, token_hash, token_type, expires_at)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetAuthTokenByHash :one
SELECT * FROM auth_tokens
WHERE token_hash = $1
  AND token_type = $2
  AND expires_at > NOW()
  AND used_at IS NULL;

-- name: MarkAuthTokenUsed :exec
UPDATE auth_tokens
SET used_at = NOW()
WHERE id = $1;

-- name: DeleteAuthToken :exec
DELETE FROM auth_tokens WHERE id = $1;

-- name: DeleteUserAuthTokens :exec
DELETE FROM auth_tokens WHERE user_id = $1 AND token_type = $2;

-- name: DeleteExpiredAuthTokens :execrows
DELETE FROM auth_tokens WHERE expires_at <= NOW();
