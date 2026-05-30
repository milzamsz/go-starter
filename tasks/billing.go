package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/hibiken/asynq"

	"github.com/milzam/go-starter/internal/queue"
)

// BillingWebhookPayload holds data for processing a Stripe webhook.
type BillingWebhookPayload struct {
	EventID   string `json:"event_id"`
	EventType string `json:"event_type"`
}

// BillingHandler handles billing-related background tasks.
type BillingHandler struct {
	logger *slog.Logger
}

// NewBillingHandler creates a new billing task handler.
func NewBillingHandler(logger *slog.Logger) *BillingHandler {
	return &BillingHandler{
		logger: logger,
	}
}

// HandleProcessStripeWebhook processes a Stripe webhook event asynchronously.
func (h *BillingHandler) HandleProcessStripeWebhook(ctx context.Context, t *asynq.Task) error {
	var payload BillingWebhookPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	h.logger.InfoContext(ctx, "processing stripe webhook",
		slog.String("event_id", payload.EventID),
		slog.String("event_type", payload.EventType),
	)

	return nil
}

// NewProcessStripeWebhookTask creates a task for processing a Stripe webhook.
func NewProcessStripeWebhookTask(eventID, eventType string) (*asynq.Task, error) {
	payload, err := json.Marshal(BillingWebhookPayload{
		EventID:   eventID,
		EventType: eventType,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}
	return asynq.NewTask(queue.TaskProcessStripeWebhook, payload), nil
}
