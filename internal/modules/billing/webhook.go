package billing

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/webhook"

	appErrors "github.com/milzam/go-starter/internal/errors"
	"github.com/milzam/go-starter/internal/sqlc"
)

// HandleStripeWebhook handles incoming webhooks from Stripe with signature verification and idempotency check.
// POST /webhooks/stripe
func (h *Handlers) HandleStripeWebhook(w http.ResponseWriter, r *http.Request) {
	const maxBodySize = int64(65536) // 64 KB limit
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

	payload, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "reading webhook payload", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	sig := r.Header.Get("Stripe-Signature")
	if sig == "" {
		h.logger.WarnContext(r.Context(), "missing Stripe-Signature header")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Verify cryptographic signature
	event, err := webhook.ConstructEvent(payload, sig, h.cfg.WebhookSecret)
	if err != nil {
		h.logger.WarnContext(r.Context(), "invalid Stripe webhook signature", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Idempotency: Check if the event has already been registered.
	dbEvent, err := h.service.queries.GetWebhookEventByEventID(ctx, event.ID)
	if err != nil && err != pgx.ErrNoRows {
		h.logger.ErrorContext(ctx, "checking webhook event in database", "event_id", event.ID, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if err == nil {
		// Event already exists.
		if dbEvent.Processed {
			h.logger.InfoContext(ctx, "webhook event already processed (idempotency)", "event_id", event.ID)
			w.WriteHeader(http.StatusOK)
			return
		}
		// Event is in database but not processed yet. We can fail and let Stripe retry.
		h.logger.WarnContext(ctx, "webhook event in database but not yet fully processed", "event_id", event.ID)
		w.WriteHeader(http.StatusAccepted)
		return
	}

	// Save the unprocessed webhook event in the database for tracking.
	rawPayload, err := json.Marshal(event)
	if err != nil {
		h.logger.ErrorContext(ctx, "failed to marshal event for logging", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	savedEvent, err := h.service.queries.CreateWebhookEvent(ctx, sqlc.CreateWebhookEventParams{
		EventID:   event.ID,
		EventType: string(event.Type),
		Payload:   rawPayload,
	})
	if err != nil {
		h.logger.ErrorContext(ctx, "failed to store webhook event", "event_id", event.ID, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Handle the event types
	var processErr error

	switch event.Type {
	case "checkout.session.completed":
		var sess stripe.CheckoutSession
		if err := json.Unmarshal(event.Data.Raw, &sess); err == nil {
			if sess.Subscription != nil {
				processErr = h.service.SyncSubscriptionStatus(ctx, sess.Subscription.ID)
			}
		} else {
			processErr = err
		}

	case "customer.subscription.updated", "customer.subscription.deleted":
		var sub stripe.Subscription
		if err := json.Unmarshal(event.Data.Raw, &sub); err == nil {
			processErr = h.service.SyncSubscriptionStatus(ctx, sub.ID)
		} else {
			processErr = err
		}

	case "invoice.payment_failed":
		var inv stripe.Invoice
		if err := json.Unmarshal(event.Data.Raw, &inv); err == nil {
			if inv.Subscription != nil {
				processErr = h.service.SyncSubscriptionStatus(ctx, inv.Subscription.ID)
			}
		} else {
			processErr = err
		}
	}

	if processErr != nil {
		h.logger.ErrorContext(ctx, "failed to process webhook event", "event_id", event.ID, "error", processErr)
		// Update DB with error description
		errMsg := processErr.Error()
		_ = h.service.queries.MarkWebhookError(ctx, sqlc.MarkWebhookErrorParams{
			ID:    savedEvent.ID,
			Error: &errMsg,
		})

		// Return 500 so Stripe retries
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Mark the event as processed successfully
	err = h.service.queries.MarkWebhookProcessed(ctx, savedEvent.ID)
	if err != nil {
		h.logger.ErrorContext(ctx, "failed to mark webhook event as processed", "event_id", event.ID, "error", err)
		// Return 200 anyway since the business action completed successfully
	}

	h.logger.InfoContext(ctx, "webhook event processed successfully", "event_id", event.ID, "type", event.Type)
	w.WriteHeader(http.StatusOK)
}

// respondError helper for JSON responses inside billing handlers.
func (h *Handlers) respondError(w http.ResponseWriter, err error) {
	var appErr *appErrors.AppError
	if !errors.As(err, &appErr) {
		appErr = appErrors.NewInternal("an unexpected error occurred", err)
	}

	status := appErrors.HTTPStatus(appErr)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if encodeErr := json.NewEncoder(w).Encode(appErr); encodeErr != nil {
		h.logger.Error("failed to encode billing error response", "error", encodeErr)
	}
}
