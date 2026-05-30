package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/milzam/go-starter/internal/config"
	appErrors "github.com/milzam/go-starter/internal/errors"
	"github.com/milzam/go-starter/internal/queue"
	"github.com/milzam/go-starter/internal/sqlc"
	"github.com/milzam/go-starter/tasks"
)

// Service handles all authentication and authorization business logic.
type Service struct {
	queries        *sqlc.Queries
	db             *pgxpool.Pool
	jwt            *JWTManager
	logger         *slog.Logger
	cfg            config.AuthConfig
	oauthProviders map[string]OAuthProvider
	queueClient    *queue.Client
}

// NewService creates a new auth Service with the given dependencies.
func NewService(queries *sqlc.Queries, db *pgxpool.Pool, jwt *JWTManager, logger *slog.Logger, cfg config.AuthConfig, queueClient *queue.Client) *Service {
	return &Service{
		queries:        queries,
		db:             db,
		jwt:            jwt,
		logger:         logger.With("component", "auth.Service"),
		cfg:            cfg,
		oauthProviders: make(map[string]OAuthProvider),
		queueClient:    queueClient,
	}
}

// AddOAuthProvider registers an OAuth provider by its name.
func (s *Service) AddOAuthProvider(provider OAuthProvider) {
	s.oauthProviders[provider.Name()] = provider
}

// Signup creates a new user account with email and password.
func (s *Service) Signup(ctx context.Context, req SignupRequest) (*AuthResponse, error) {
	// Check if the email is already registered.
	_, err := s.queries.GetUserByEmail(ctx, req.Email)
	if err == nil {
		return nil, appErrors.NewConflict("email already registered")
	}
	if err != pgx.ErrNoRows {
		s.logger.ErrorContext(ctx, "checking existing user", "error", err)
		return nil, appErrors.NewInternal("failed to check existing user", err)
	}

	// Hash the password.
	hash, err := hashPassword(req.Password, s.cfg.BcryptCost)
	if err != nil {
		s.logger.ErrorContext(ctx, "hashing password", "error", err)
		return nil, appErrors.NewInternal("failed to hash password", err)
	}

	// Create the user in the database.
	user, err := s.queries.CreateUser(ctx, sqlc.CreateUserParams{
		Email:        req.Email,
		PasswordHash: &hash,
		Name:         req.Name,
		Role:         sqlc.UserRoleUser,
	})
	if err != nil {
		s.logger.ErrorContext(ctx, "creating user", "error", err)
		return nil, appErrors.NewInternal("failed to create user", err)
	}

	// Generate an email verification token and store the hash.
	rawToken, hashedToken, err := generateToken()
	if err != nil {
		s.logger.ErrorContext(ctx, "generating verification token", "error", err)
		return nil, appErrors.NewInternal("failed to generate verification token", err)
	}

	_, err = s.queries.CreateAuthToken(ctx, sqlc.CreateAuthTokenParams{
		UserID:    user.ID,
		TokenHash: hashedToken,
		TokenType: sqlc.TokenTypeVerification,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	})
	if err != nil {
		s.logger.ErrorContext(ctx, "storing verification token", "error", err)
		return nil, appErrors.NewInternal("failed to store verification token", err)
	}

	if s.queueClient != nil {
		task, taskErr := tasks.NewSendVerificationEmailTask(user.ID.String(), user.Email, user.Name, rawToken)
		if taskErr == nil {
			_, _ = s.queueClient.Enqueue(task)
		}
	}

	s.logger.InfoContext(ctx, "user signed up", "user_id", user.ID, "email", user.Email)

	return s.generateAuthResponse(user)
}

// Login authenticates a user with email/password and optional TOTP code.
func (s *Service) Login(ctx context.Context, req LoginRequest) (*AuthResponse, error) {
	user, err := s.queries.GetUserByEmail(ctx, req.Email)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, appErrors.NewUnauthorized("invalid email or password")
		}
		s.logger.ErrorContext(ctx, "getting user by email", "error", err)
		return nil, appErrors.NewInternal("failed to get user", err)
	}

	// OAuth-only users cannot log in with a password.
	if user.PasswordHash == nil {
		return nil, appErrors.NewUnauthorized("this account uses social login")
	}

	if err := checkPassword(*user.PasswordHash, req.Password); err != nil {
		return nil, appErrors.NewUnauthorized("invalid email or password")
	}

	// If 2FA is enabled, require a valid TOTP code.
	if user.TotpEnabled {
		if req.TOTPCode == "" {
			return nil, appErrors.NewValidation("TOTP code is required").WithDetail("totp_required")
		}
		if user.TotpSecret == nil || !ValidateTOTPCode(*user.TotpSecret, req.TOTPCode) {
			return nil, appErrors.NewUnauthorized("invalid TOTP code")
		}
	}

	s.logger.InfoContext(ctx, "user logged in", "user_id", user.ID, "email", user.Email)

	return s.generateAuthResponse(user)
}

// RefreshToken generates a new access/refresh token pair from a valid refresh token.
func (s *Service) RefreshToken(ctx context.Context, refreshToken string) (*AuthResponse, error) {
	claims, err := s.jwt.ValidateToken(refreshToken)
	if err != nil {
		return nil, appErrors.NewUnauthorized("invalid or expired refresh token")
	}

	user, err := s.queries.GetUserByID(ctx, claims.UserID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, appErrors.NewUnauthorized("user not found")
		}
		s.logger.ErrorContext(ctx, "getting user for refresh", "error", err)
		return nil, appErrors.NewInternal("failed to get user", err)
	}

	return s.generateAuthResponse(user)
}

// VerifyEmail marks the user's email as verified using a one-time token.
func (s *Service) VerifyEmail(ctx context.Context, token string) error {
	hashed := hashTokenValue(token)

	authToken, err := s.queries.GetAuthTokenByHash(ctx, sqlc.GetAuthTokenByHashParams{
		TokenHash: hashed,
		TokenType: sqlc.TokenTypeVerification,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return appErrors.NewBadRequest("invalid or expired verification token")
		}
		s.logger.ErrorContext(ctx, "getting verification token", "error", err)
		return appErrors.NewInternal("failed to verify email", err)
	}

	// Mark the token as used.
	if err := s.queries.MarkAuthTokenUsed(ctx, authToken.ID); err != nil {
		s.logger.ErrorContext(ctx, "marking token used", "error", err)
		return appErrors.NewInternal("failed to mark token used", err)
	}

	// Update the user's email_verified flag.
	if err := s.queries.UpdateUserEmailVerified(ctx, sqlc.UpdateUserEmailVerifiedParams{
		ID:            authToken.UserID,
		EmailVerified: true,
	}); err != nil {
		s.logger.ErrorContext(ctx, "updating email verified", "error", err)
		return appErrors.NewInternal("failed to verify email", err)
	}

	s.logger.InfoContext(ctx, "email verified", "user_id", authToken.UserID)
	return nil
}

// ForgotPassword generates a password reset token and enqueues an email.
// It always returns nil to avoid leaking whether the email exists.
func (s *Service) ForgotPassword(ctx context.Context, email string) error {
	user, err := s.queries.GetUserByEmail(ctx, email)
	if err != nil {
		if err == pgx.ErrNoRows {
			// Don't reveal whether the email exists.
			s.logger.InfoContext(ctx, "forgot password for unknown email", "email", email)
			return nil
		}
		s.logger.ErrorContext(ctx, "getting user for password reset", "error", err)
		return appErrors.NewInternal("failed to process request", err)
	}

	// Delete any existing password reset tokens for this user.
	if err := s.queries.DeleteUserAuthTokens(ctx, sqlc.DeleteUserAuthTokensParams{
		UserID:    user.ID,
		TokenType: sqlc.TokenTypePasswordReset,
	}); err != nil {
		s.logger.ErrorContext(ctx, "deleting old reset tokens", "error", err)
		return appErrors.NewInternal("failed to process request", err)
	}

	// Generate a new token.
	rawToken, hashedToken, err := generateToken()
	if err != nil {
		s.logger.ErrorContext(ctx, "generating reset token", "error", err)
		return appErrors.NewInternal("failed to generate reset token", err)
	}

	_, err = s.queries.CreateAuthToken(ctx, sqlc.CreateAuthTokenParams{
		UserID:    user.ID,
		TokenHash: hashedToken,
		TokenType: sqlc.TokenTypePasswordReset,
		ExpiresAt: time.Now().Add(1 * time.Hour),
	})
	if err != nil {
		s.logger.ErrorContext(ctx, "storing reset token", "error", err)
		return appErrors.NewInternal("failed to store reset token", err)
	}

	if s.queueClient != nil {
		task, taskErr := tasks.NewSendResetEmailTask(user.ID.String(), user.Email, user.Name, rawToken)
		if taskErr == nil {
			_, _ = s.queueClient.Enqueue(task)
		}
	}

	s.logger.InfoContext(ctx, "password reset token generated", "user_id", user.ID)
	return nil
}

// ResetPassword sets a new password using a valid password reset token.
func (s *Service) ResetPassword(ctx context.Context, req ResetPasswordRequest) error {
	hashed := hashTokenValue(req.Token)

	authToken, err := s.queries.GetAuthTokenByHash(ctx, sqlc.GetAuthTokenByHashParams{
		TokenHash: hashed,
		TokenType: sqlc.TokenTypePasswordReset,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return appErrors.NewBadRequest("invalid or expired reset token")
		}
		s.logger.ErrorContext(ctx, "getting reset token", "error", err)
		return appErrors.NewInternal("failed to reset password", err)
	}

	// Hash the new password.
	newHash, err := hashPassword(req.NewPassword, s.cfg.BcryptCost)
	if err != nil {
		s.logger.ErrorContext(ctx, "hashing new password", "error", err)
		return appErrors.NewInternal("failed to hash password", err)
	}

	// Update the user's password.
	if err := s.queries.UpdateUserPassword(ctx, sqlc.UpdateUserPasswordParams{
		ID:           authToken.UserID,
		PasswordHash: &newHash,
	}); err != nil {
		s.logger.ErrorContext(ctx, "updating password", "error", err)
		return appErrors.NewInternal("failed to update password", err)
	}

	// Mark the reset token as used.
	if err := s.queries.MarkAuthTokenUsed(ctx, authToken.ID); err != nil {
		s.logger.ErrorContext(ctx, "marking reset token used", "error", err)
		// Non-critical: password is already updated.
	}

	// Delete any remaining password reset tokens for this user.
	if err := s.queries.DeleteUserAuthTokens(ctx, sqlc.DeleteUserAuthTokensParams{
		UserID:    authToken.UserID,
		TokenType: sqlc.TokenTypePasswordReset,
	}); err != nil {
		s.logger.ErrorContext(ctx, "deleting old reset tokens", "error", err)
		// Non-critical: tokens will expire naturally.
	}

	s.logger.InfoContext(ctx, "password reset", "user_id", authToken.UserID)
	return nil
}

// GetCurrentUser returns the profile of the authenticated user.
func (s *Service) GetCurrentUser(ctx context.Context, userID uuid.UUID) (*UserInfo, error) {
	user, err := s.queries.GetUserByID(ctx, userID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, appErrors.NewNotFound("user not found")
		}
		s.logger.ErrorContext(ctx, "getting current user", "error", err)
		return nil, appErrors.NewInternal("failed to get user", err)
	}

	info := userToInfo(user)
	return &info, nil
}

// ChangePassword changes the authenticated user's password after verifying the current one.
func (s *Service) ChangePassword(ctx context.Context, userID uuid.UUID, req ChangePasswordRequest) error {
	user, err := s.queries.GetUserByID(ctx, userID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return appErrors.NewNotFound("user not found")
		}
		s.logger.ErrorContext(ctx, "getting user for password change", "error", err)
		return appErrors.NewInternal("failed to get user", err)
	}

	// OAuth-only users cannot change a password they don't have.
	if user.PasswordHash == nil {
		return appErrors.NewBadRequest("account uses social login, no password to change")
	}

	if err := checkPassword(*user.PasswordHash, req.CurrentPassword); err != nil {
		return appErrors.NewUnauthorized("current password is incorrect")
	}

	newHash, err := hashPassword(req.NewPassword, s.cfg.BcryptCost)
	if err != nil {
		s.logger.ErrorContext(ctx, "hashing new password", "error", err)
		return appErrors.NewInternal("failed to hash password", err)
	}

	if err := s.queries.UpdateUserPassword(ctx, sqlc.UpdateUserPasswordParams{
		ID:           userID,
		PasswordHash: &newHash,
	}); err != nil {
		s.logger.ErrorContext(ctx, "updating password", "error", err)
		return appErrors.NewInternal("failed to update password", err)
	}

	s.logger.InfoContext(ctx, "password changed", "user_id", userID)
	return nil
}

// Enable2FA generates a TOTP secret for the user and returns it for QR provisioning.
func (s *Service) Enable2FA(ctx context.Context, userID uuid.UUID) (*Enable2FAResponse, error) {
	user, err := s.queries.GetUserByID(ctx, userID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, appErrors.NewNotFound("user not found")
		}
		s.logger.ErrorContext(ctx, "getting user for 2FA setup", "error", err)
		return nil, appErrors.NewInternal("failed to get user", err)
	}

	if user.TotpEnabled {
		return nil, appErrors.NewConflict("2FA is already enabled")
	}

	secret, qrURL, err := GenerateTOTPSecret(user.Email, "go-starter")
	if err != nil {
		s.logger.ErrorContext(ctx, "generating TOTP secret", "error", err)
		return nil, appErrors.NewInternal("failed to generate 2FA secret", err)
	}

	// Store the secret but don't enable 2FA yet — wait for verification.
	if err := s.queries.UpdateUserTOTP(ctx, sqlc.UpdateUserTOTPParams{
		ID:          userID,
		TotpSecret:  &secret,
		TotpEnabled: false,
	}); err != nil {
		s.logger.ErrorContext(ctx, "storing TOTP secret", "error", err)
		return nil, appErrors.NewInternal("failed to store 2FA secret", err)
	}

	return &Enable2FAResponse{
		Secret: secret,
		QRURL:  qrURL,
	}, nil
}

// Verify2FA confirms the TOTP setup by validating a code and enabling 2FA.
func (s *Service) Verify2FA(ctx context.Context, userID uuid.UUID, code string) error {
	user, err := s.queries.GetUserByID(ctx, userID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return appErrors.NewNotFound("user not found")
		}
		s.logger.ErrorContext(ctx, "getting user for 2FA verification", "error", err)
		return appErrors.NewInternal("failed to get user", err)
	}

	if user.TotpEnabled {
		return appErrors.NewConflict("2FA is already enabled")
	}

	if user.TotpSecret == nil {
		return appErrors.NewBadRequest("2FA setup has not been initiated")
	}

	if !ValidateTOTPCode(*user.TotpSecret, code) {
		return appErrors.NewUnauthorized("invalid TOTP code")
	}

	// Enable 2FA.
	if err := s.queries.UpdateUserTOTP(ctx, sqlc.UpdateUserTOTPParams{
		ID:          userID,
		TotpSecret:  user.TotpSecret,
		TotpEnabled: true,
	}); err != nil {
		s.logger.ErrorContext(ctx, "enabling 2FA", "error", err)
		return appErrors.NewInternal("failed to enable 2FA", err)
	}

	s.logger.InfoContext(ctx, "2FA enabled", "user_id", userID)
	return nil
}

// Disable2FA removes 2FA from the user's account.
func (s *Service) Disable2FA(ctx context.Context, userID uuid.UUID) error {
	user, err := s.queries.GetUserByID(ctx, userID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return appErrors.NewNotFound("user not found")
		}
		s.logger.ErrorContext(ctx, "getting user for 2FA disable", "error", err)
		return appErrors.NewInternal("failed to get user", err)
	}

	if !user.TotpEnabled {
		return appErrors.NewBadRequest("2FA is not enabled")
	}

	if err := s.queries.UpdateUserTOTP(ctx, sqlc.UpdateUserTOTPParams{
		ID:          userID,
		TotpSecret:  nil,
		TotpEnabled: false,
	}); err != nil {
		s.logger.ErrorContext(ctx, "disabling 2FA", "error", err)
		return appErrors.NewInternal("failed to disable 2FA", err)
	}

	s.logger.InfoContext(ctx, "2FA disabled", "user_id", userID)
	return nil
}

// OAuthLogin handles the OAuth callback flow: exchanges the code for a token,
// fetches user info from the provider, and finds or creates a local user.
func (s *Service) OAuthLogin(ctx context.Context, providerName, code string) (*AuthResponse, error) {
	provider, ok := s.oauthProviders[providerName]
	if !ok {
		return nil, appErrors.NewBadRequest(fmt.Sprintf("unsupported OAuth provider: %s", providerName))
	}

	// Exchange the authorization code for tokens.
	oauthToken, err := provider.Exchange(ctx, code)
	if err != nil {
		s.logger.ErrorContext(ctx, "OAuth token exchange failed", "provider", providerName, "error", err)
		return nil, appErrors.NewUnauthorized("OAuth authentication failed")
	}

	// Fetch user info from the provider.
	oauthUser, err := provider.GetUserInfo(ctx, oauthToken)
	if err != nil {
		s.logger.ErrorContext(ctx, "fetching OAuth user info", "provider", providerName, "error", err)
		return nil, appErrors.NewInternal("failed to get user info from provider", err)
	}

	if oauthUser.Email == "" {
		return nil, appErrors.NewBadRequest("email not available from OAuth provider")
	}

	// Check if we have an existing OAuth identity for this provider+providerID.
	oauthIdentity, err := s.queries.GetOAuthIdentity(ctx, sqlc.GetOAuthIdentityParams{
		Provider:       sqlc.OauthProvider(providerName),
		ProviderUserID: oauthUser.ProviderID,
	})
	if err != nil && err != pgx.ErrNoRows {
		s.logger.ErrorContext(ctx, "checking OAuth identity", "error", err)
		return nil, appErrors.NewInternal("failed to check OAuth identity", err)
	}

	var user sqlc.User

	if err == nil {
		// Existing OAuth identity — fetch the linked user.
		user, err = s.queries.GetUserByID(ctx, oauthIdentity.UserID)
		if err != nil {
			s.logger.ErrorContext(ctx, "getting user for OAuth identity", "error", err)
			return nil, appErrors.NewInternal("failed to get user", err)
		}

		// Update OAuth tokens.
		var refreshTokenStr *string
		if oauthToken.RefreshToken != "" {
			refreshTokenStr = &oauthToken.RefreshToken
		}
		var expiresAt pgtype.Timestamptz
		if !oauthToken.Expiry.IsZero() {
			expiresAt = pgtype.Timestamptz{Time: oauthToken.Expiry, Valid: true}
		}
		_ = s.queries.UpdateOAuthTokens(ctx, sqlc.UpdateOAuthTokensParams{
			ID:           oauthIdentity.ID,
			AccessToken:  &oauthToken.AccessToken,
			RefreshToken: refreshTokenStr,
			ExpiresAt:    expiresAt,
		})
	} else {
		// No OAuth identity found. Check if a user with this email exists.
		user, err = s.queries.GetUserByEmail(ctx, oauthUser.Email)
		if err != nil && err != pgx.ErrNoRows {
			s.logger.ErrorContext(ctx, "checking email for OAuth", "error", err)
			return nil, appErrors.NewInternal("failed to check user email", err)
		}

		if err == pgx.ErrNoRows {
			// Create a new user (no password — OAuth-only).
			user, err = s.queries.CreateUser(ctx, sqlc.CreateUserParams{
				Email:        oauthUser.Email,
				PasswordHash: nil,
				Name:         oauthUser.Name,
				Role:         sqlc.UserRoleUser,
			})
			if err != nil {
				s.logger.ErrorContext(ctx, "creating OAuth user", "error", err)
				return nil, appErrors.NewInternal("failed to create user", err)
			}

			// Mark email as verified since it came from a trusted provider.
			_ = s.queries.UpdateUserEmailVerified(ctx, sqlc.UpdateUserEmailVerifiedParams{
				ID:            user.ID,
				EmailVerified: true,
			})
			user.EmailVerified = true
		}

		// Link the OAuth identity to the user.
		var refreshTokenStr *string
		if oauthToken.RefreshToken != "" {
			refreshTokenStr = &oauthToken.RefreshToken
		}
		var expiresAt pgtype.Timestamptz
		if !oauthToken.Expiry.IsZero() {
			expiresAt = pgtype.Timestamptz{Time: oauthToken.Expiry, Valid: true}
		}
		_, err = s.queries.CreateOAuthIdentity(ctx, sqlc.CreateOAuthIdentityParams{
			UserID:         user.ID,
			Provider:       sqlc.OauthProvider(providerName),
			ProviderUserID: oauthUser.ProviderID,
			AccessToken:    &oauthToken.AccessToken,
			RefreshToken:   refreshTokenStr,
			ExpiresAt:      expiresAt,
		})
		if err != nil {
			s.logger.ErrorContext(ctx, "creating OAuth identity", "error", err)
			return nil, appErrors.NewInternal("failed to link OAuth identity", err)
		}
	}

	s.logger.InfoContext(ctx, "OAuth login", "provider", providerName, "user_id", user.ID)

	return s.generateAuthResponse(user)
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// generateAuthResponse creates an AuthResponse with fresh JWT tokens for the user.
func (s *Service) generateAuthResponse(user sqlc.User) (*AuthResponse, error) {
	accessToken, expiresAt, err := s.jwt.GenerateAccessToken(user.ID, user.Email, string(user.Role))
	if err != nil {
		s.logger.Error("generating access token", "error", err)
		return nil, appErrors.NewInternal("failed to generate access token", err)
	}

	refreshToken, _, err := s.jwt.GenerateRefreshToken(user.ID)
	if err != nil {
		s.logger.Error("generating refresh token", "error", err)
		return nil, appErrors.NewInternal("failed to generate refresh token", err)
	}

	return &AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    expiresAt,
		User:         userToInfo(user),
	}, nil
}

// hashPassword hashes a plaintext password using bcrypt at the specified cost.
func hashPassword(password string, cost int) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	if err != nil {
		return "", fmt.Errorf("bcrypt hash: %w", err)
	}
	return string(bytes), nil
}

// checkPassword compares a bcrypt hash with a plaintext password.
func checkPassword(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

// generateToken creates a cryptographically random 32-byte token and returns
// both the raw hex string (to send to the user) and its SHA-256 hash (to store).
func generateToken() (raw string, hashed string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("generating random token: %w", err)
	}
	rawHex := hex.EncodeToString(b)
	return rawHex, hashTokenValue(rawHex), nil
}

// hashTokenValue computes the SHA-256 hex digest of a token string.
// Only the hash is stored in the database; the raw token is sent to the user.
func hashTokenValue(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// userToInfo converts a database User row to a sanitized UserInfo response.
func userToInfo(u sqlc.User) UserInfo {
	return UserInfo{
		ID:            u.ID,
		Email:         u.Email,
		Name:          u.Name,
		Role:          string(u.Role),
		EmailVerified: u.EmailVerified,
		TOTPEnabled:   u.TotpEnabled,
		Plan:          u.Plan,
		SubStatus:     string(u.SubscriptionStatus),
		CreatedAt:     u.CreatedAt,
	}
}
