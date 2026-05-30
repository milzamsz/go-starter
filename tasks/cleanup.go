package tasks

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/milzam/go-starter/internal/queue"
)

// CleanupHandler handles periodic cleanup tasks.
type CleanupHandler struct {
	db     *pgxpool.Pool
	logger *slog.Logger
}

// NewCleanupHandler creates a new cleanup task handler.
func NewCleanupHandler(db *pgxpool.Pool, logger *slog.Logger) *CleanupHandler {
	return &CleanupHandler{
		db:     db,
		logger: logger,
	}
}

// HandleCleanupExpiredTokens removes expired auth tokens from the database.
func (h *CleanupHandler) HandleCleanupExpiredTokens(ctx context.Context, t *asynq.Task) error {
	h.logger.InfoContext(ctx, "starting expired token cleanup")

	result, err := h.db.Exec(ctx, "DELETE FROM auth_tokens WHERE expires_at <= NOW()")
	if err != nil {
		return fmt.Errorf("delete expired tokens: %w", err)
	}

	h.logger.InfoContext(ctx, "expired token cleanup complete",
		slog.Int64("deleted", result.RowsAffected()),
	)
	return nil
}

// HandleCleanupExpiredSessions removes expired sessions from the database.
func (h *CleanupHandler) HandleCleanupExpiredSessions(ctx context.Context, t *asynq.Task) error {
	h.logger.InfoContext(ctx, "starting expired session cleanup")

	result, err := h.db.Exec(ctx, "DELETE FROM sessions WHERE expires_at <= NOW()")
	if err != nil {
		return fmt.Errorf("delete expired sessions: %w", err)
	}

	h.logger.InfoContext(ctx, "expired session cleanup complete",
		slog.Int64("deleted", result.RowsAffected()),
	)
	return nil
}

// NewCleanupExpiredTokensTask creates a task for cleaning expired tokens.
func NewCleanupExpiredTokensTask() *asynq.Task {
	return asynq.NewTask(queue.TaskCleanupExpiredTokens, nil)
}

// NewCleanupExpiredSessionsTask creates a task for cleaning expired sessions.
func NewCleanupExpiredSessionsTask() *asynq.Task {
	return asynq.NewTask(queue.TaskCleanupExpiredSessions, nil)
}
