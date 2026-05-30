-- name: CreateUser :one
INSERT INTO users (email, password_hash, name, role)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: UpdateUser :one
UPDATE users
SET name = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: UpdateUserPassword :exec
UPDATE users
SET password_hash = $2, updated_at = NOW()
WHERE id = $1;

-- name: UpdateUserRole :exec
UPDATE users
SET role = $2, updated_at = NOW()
WHERE id = $1;

-- name: UpdateUserEmailVerified :exec
UPDATE users
SET email_verified = $2, updated_at = NOW()
WHERE id = $1;

-- name: UpdateUserTOTP :exec
UPDATE users
SET totp_secret = $2, totp_enabled = $3, updated_at = NOW()
WHERE id = $1;

-- name: UpdateUserBilling :exec
UPDATE users
SET stripe_customer_id = $2,
    plan = $3,
    subscription_status = $4,
    subscription_id = $5,
    updated_at = NOW()
WHERE id = $1;

-- name: ListUsers :many
SELECT * FROM users
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountUsers :one
SELECT COUNT(*) FROM users;

-- name: DeleteUser :exec
DELETE FROM users WHERE id = $1;
