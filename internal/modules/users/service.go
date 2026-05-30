package users

import (
	"context"
	"log/slog"
	"math"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	appErrors "github.com/milzam/go-starter/internal/errors"
	"github.com/milzam/go-starter/internal/sqlc"
)

// Service provides user management functionality.
type Service struct {
	queries *sqlc.Queries
	db      *pgxpool.Pool
	logger  *slog.Logger
}

// NewService creates a new user Service with its required dependencies.
func NewService(queries *sqlc.Queries, db *pgxpool.Pool, logger *slog.Logger) *Service {
	return &Service{
		queries: queries,
		db:      db,
		logger:  logger.With("component", "users.Service"),
	}
}

// GetUser retrieves a user by ID.
func (s *Service) GetUser(ctx context.Context, id uuid.UUID) (*User, error) {
	row, err := s.queries.GetUserByID(ctx, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, appErrors.NewNotFound("user not found")
		}
		s.logger.ErrorContext(ctx, "getting user by ID", "user_id", id, "error", err)
		return nil, appErrors.NewInternal("failed to retrieve user", err)
	}

	u := sqlcToUser(row)
	return &u, nil
}

// ListUsers retrieves a paginated list of users (primarily for admin use).
func (s *Service) ListUsers(ctx context.Context, page, pageSize int) (*ListUsersResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	limit := int32(pageSize)
	offset := int32((page - 1) * pageSize)

	rows, err := s.queries.ListUsers(ctx, sqlc.ListUsersParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		s.logger.ErrorContext(ctx, "listing users", "error", err)
		return nil, appErrors.NewInternal("failed to list users", err)
	}

	total, err := s.queries.CountUsers(ctx)
	if err != nil {
		s.logger.ErrorContext(ctx, "counting users", "error", err)
		return nil, appErrors.NewInternal("failed to count users", err)
	}

	usersList := make([]User, len(rows))
	for i, r := range rows {
		usersList[i] = sqlcToUser(r)
	}

	totalPages := int(math.Ceil(float64(total) / float64(pageSize)))

	return &ListUsersResponse{
		Users:      usersList,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

// UpdateUser updates a user's editable profile fields.
func (s *Service) UpdateUser(ctx context.Context, id uuid.UUID, req UpdateUserRequest) (*User, error) {
	// First check if user exists.
	_, err := s.queries.GetUserByID(ctx, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, appErrors.NewNotFound("user not found")
		}
		s.logger.ErrorContext(ctx, "getting user for update check", "user_id", id, "error", err)
		return nil, appErrors.NewInternal("failed to update user", err)
	}

	row, err := s.queries.UpdateUser(ctx, sqlc.UpdateUserParams{
		ID:   id,
		Name: req.Name,
	})
	if err != nil {
		s.logger.ErrorContext(ctx, "updating user profile", "user_id", id, "error", err)
		return nil, appErrors.NewInternal("failed to update user profile", err)
	}

	s.logger.InfoContext(ctx, "user profile updated", "user_id", id)

	u := sqlcToUser(row)
	return &u, nil
}

// UpdateUserRole updates a user's global authorization role (admin operation).
func (s *Service) UpdateUserRole(ctx context.Context, id uuid.UUID, role string) error {
	// First check if user exists.
	_, err := s.queries.GetUserByID(ctx, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return appErrors.NewNotFound("user not found")
		}
		s.logger.ErrorContext(ctx, "getting user for role update check", "user_id", id, "error", err)
		return appErrors.NewInternal("failed to update user role", err)
	}

	err = s.queries.UpdateUserRole(ctx, sqlc.UpdateUserRoleParams{
		ID:   id,
		Role: sqlc.UserRole(role),
	})
	if err != nil {
		s.logger.ErrorContext(ctx, "updating user role", "user_id", id, "role", role, "error", err)
		return appErrors.NewInternal("failed to update user role", err)
	}

	s.logger.InfoContext(ctx, "user role updated", "user_id", id, "role", role)
	return nil
}

// DeleteUser deletes a user account entirely (admin operation).
func (s *Service) DeleteUser(ctx context.Context, id uuid.UUID) error {
	// First check if user exists.
	_, err := s.queries.GetUserByID(ctx, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return appErrors.NewNotFound("user not found")
		}
		s.logger.ErrorContext(ctx, "getting user for deletion check", "user_id", id, "error", err)
		return appErrors.NewInternal("failed to delete user", err)
	}

	err = s.queries.DeleteUser(ctx, id)
	if err != nil {
		s.logger.ErrorContext(ctx, "deleting user account", "user_id", id, "error", err)
		return appErrors.NewInternal("failed to delete user", err)
	}

	s.logger.InfoContext(ctx, "user deleted", "user_id", id)
	return nil
}

// sqlcToUser converts a sqlc.User DB row into a clean users.User struct.
func sqlcToUser(u sqlc.User) User {
	var stripeSubStatus string
	if u.SubscriptionStatus != "" {
		stripeSubStatus = string(u.SubscriptionStatus)
	}
	return User{
		ID:                 u.ID,
		Email:              u.Email,
		Name:               u.Name,
		Role:               string(u.Role),
		EmailVerified:      u.EmailVerified,
		TOTPEnabled:        u.TotpEnabled,
		Plan:               u.Plan,
		SubscriptionStatus: stripeSubStatus,
		CreatedAt:          u.CreatedAt,
		UpdatedAt:          u.UpdatedAt,
	}
}
