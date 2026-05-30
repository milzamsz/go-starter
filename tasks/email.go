package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/hibiken/asynq"

	"github.com/milzam/go-starter/internal/email"
	"github.com/milzam/go-starter/internal/queue"
)

// EmailPayload holds the data needed to send an email.
type EmailPayload struct {
	UserID    string `json:"user_id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	Token     string `json:"token,omitempty"`
	EmailType string `json:"email_type"`
}

// EmailHandler handles email sending tasks.
type EmailHandler struct {
	logger *slog.Logger
	sender email.Sender
}

// NewEmailHandler creates a new email task handler.
func NewEmailHandler(logger *slog.Logger, sender email.Sender) *EmailHandler {
	return &EmailHandler{
		logger: logger,
		sender: sender,
	}
}

// HandleSendVerificationEmail processes verification email tasks.
func (h *EmailHandler) HandleSendVerificationEmail(ctx context.Context, t *asynq.Task) error {
	var payload EmailPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	h.logger.InfoContext(ctx, "sending verification email",
		slog.String("email", payload.Email),
		slog.String("user_id", payload.UserID),
	)

	if h.sender == nil {
		return nil
	}
	return h.sender.Send(ctx, payload.Email, "Verify your email", fmt.Sprintf("Hello %s,\n\nVerify token: %s\n", payload.Name, payload.Token))
}

// HandleSendResetEmail processes password reset email tasks.
func (h *EmailHandler) HandleSendResetEmail(ctx context.Context, t *asynq.Task) error {
	var payload EmailPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	h.logger.InfoContext(ctx, "sending password reset email",
		slog.String("email", payload.Email),
		slog.String("user_id", payload.UserID),
	)

	if h.sender == nil {
		return nil
	}
	return h.sender.Send(ctx, payload.Email, "Reset your password", fmt.Sprintf("Hello %s,\n\nReset token: %s\n", payload.Name, payload.Token))
}

// HandleSendWelcomeEmail processes welcome email tasks.
func (h *EmailHandler) HandleSendWelcomeEmail(ctx context.Context, t *asynq.Task) error {
	var payload EmailPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	h.logger.InfoContext(ctx, "sending welcome email",
		slog.String("email", payload.Email),
		slog.String("user_id", payload.UserID),
	)

	if h.sender == nil {
		return nil
	}
	return h.sender.Send(ctx, payload.Email, "Welcome to GoStarter", fmt.Sprintf("Welcome %s,\n\nYour account is ready.\n", payload.Name))
}

// NewSendVerificationEmailTask creates a new verification email task.
func NewSendVerificationEmailTask(userID, email, name, token string) (*asynq.Task, error) {
	payload, err := json.Marshal(EmailPayload{
		UserID:    userID,
		Email:     email,
		Name:      name,
		Token:     token,
		EmailType: "verification",
	})
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}
	return asynq.NewTask(queue.TaskSendVerificationEmail, payload), nil
}

// NewSendResetEmailTask creates a new password reset email task.
func NewSendResetEmailTask(userID, email, name, token string) (*asynq.Task, error) {
	payload, err := json.Marshal(EmailPayload{
		UserID:    userID,
		Email:     email,
		Name:      name,
		Token:     token,
		EmailType: "reset",
	})
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}
	return asynq.NewTask(queue.TaskSendResetEmail, payload), nil
}

// NewSendWelcomeEmailTask creates a new welcome email task.
func NewSendWelcomeEmailTask(userID, email, name string) (*asynq.Task, error) {
	payload, err := json.Marshal(EmailPayload{
		UserID:    userID,
		Email:     email,
		Name:      name,
		EmailType: "welcome",
	})
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}
	return asynq.NewTask(queue.TaskSendWelcomeEmail, payload), nil
}
