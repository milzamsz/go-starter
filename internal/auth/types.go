package auth

import (
	"time"

	"github.com/google/uuid"
)

// SignupRequest represents a user registration request.
type SignupRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8,max=128"`
	Name     string `json:"name" validate:"required,min=1,max=255"`
}

// LoginRequest represents a login request.
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
	TOTPCode string `json:"totp_code,omitempty"`
}

// AuthResponse is returned on successful authentication.
type AuthResponse struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	User         UserInfo  `json:"user"`
}

// UserInfo is a sanitized user representation for API responses.
type UserInfo struct {
	ID            uuid.UUID `json:"id"`
	Email         string    `json:"email"`
	Name          string    `json:"name"`
	Role          string    `json:"role"`
	EmailVerified bool      `json:"email_verified"`
	TOTPEnabled   bool      `json:"totp_enabled"`
	Plan          string    `json:"plan"`
	SubStatus     string    `json:"subscription_status"`
	CreatedAt     time.Time `json:"created_at"`
}

// TokenRequest is used for refresh, verify, and reset token operations.
type TokenRequest struct {
	Token string `json:"token" validate:"required"`
}

// ResetPasswordRequest sets a new password using a reset token.
type ResetPasswordRequest struct {
	Token       string `json:"token" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=8,max=128"`
}

// ForgotPasswordRequest initiates a password reset flow.
type ForgotPasswordRequest struct {
	Email string `json:"email" validate:"required,email"`
}

// Enable2FAResponse returns the TOTP secret and provisioning QR URL.
type Enable2FAResponse struct {
	Secret string `json:"secret"`
	QRURL  string `json:"qr_url"`
}

// Verify2FARequest verifies a TOTP code during 2FA setup.
type Verify2FARequest struct {
	Code string `json:"code" validate:"required,len=6"`
}

// ChangePasswordRequest changes the authenticated user's password.
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" validate:"required"`
	NewPassword     string `json:"new_password" validate:"required,min=8,max=128"`
}
