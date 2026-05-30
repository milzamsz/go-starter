package users

import (
	"time"

	"github.com/google/uuid"
)

// User represents a user entity in the application.
type User struct {
	ID                 uuid.UUID `json:"id"`
	Email              string    `json:"email"`
	Name               string    `json:"name"`
	Role               string    `json:"role"`
	EmailVerified      bool      `json:"email_verified"`
	TOTPEnabled        bool      `json:"totp_enabled"`
	Plan               string    `json:"plan"`
	SubscriptionStatus string    `json:"subscription_status"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// UpdateUserRequest is the payload for updating a user's profile.
type UpdateUserRequest struct {
	Name string `json:"name" validate:"required,min=1,max=255"`
}

// UpdateUserRoleRequest is the payload for changing a user's role (admin only).
type UpdateUserRoleRequest struct {
	Role string `json:"role" validate:"required,oneof=user admin"`
}

// ListUsersRequest holds pagination parameters for listing users.
type ListUsersRequest struct {
	Page     int `json:"page" validate:"min=1"`
	PageSize int `json:"page_size" validate:"min=1,max=100"`
}

// ListUsersResponse is a paginated list of users.
type ListUsersResponse struct {
	Users      []User `json:"users"`
	Total      int64  `json:"total"`
	Page       int    `json:"page"`
	PageSize   int    `json:"page_size"`
	TotalPages int    `json:"total_pages"`
}
