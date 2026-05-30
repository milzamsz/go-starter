package billing

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"

	"github.com/milzam/go-starter/internal/config"
	appErrors "github.com/milzam/go-starter/internal/errors"
	"github.com/milzam/go-starter/internal/middleware"
)

// Handlers defines the HTTP handlers for the billing API.
type Handlers struct {
	service  *Service
	validate *validator.Validate
	logger   *slog.Logger
	cfg      config.StripeConfig
}

// NewHandlers creates a new billing Handlers instance with explicit dependencies.
func NewHandlers(service *Service, validate *validator.Validate, logger *slog.Logger, cfg config.StripeConfig) *Handlers {
	return &Handlers{
		service:  service,
		validate: validate,
		logger:   logger.With("component", "billing.Handlers"),
		cfg:      cfg,
	}
}

// HandleCreateCheckout launches a checkout flow for the authenticated user.
// POST /api/v1/billing/checkout (requires authentication)
func (h *Handlers) HandleCreateCheckout(w http.ResponseWriter, r *http.Request) {
	p, ok := middleware.GetPrincipal(r.Context())
	if !ok {
		h.respondError(w, appErrors.NewUnauthorized("authentication required"))
		return
	}

	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		h.respondError(w, appErrors.NewInternal("invalid user ID", err))
		return
	}

	var req CreateCheckoutRequest
	if err := h.decode(r, &req); err != nil {
		h.respondError(w, err)
		return
	}

	if err := h.validateStruct(req); err != nil {
		h.respondError(w, err)
		return
	}

	res, err := h.service.CreateCheckoutSession(r.Context(), userID, req)
	if err != nil {
		h.respondError(w, err)
		return
	}

	h.respond(w, http.StatusOK, res)
}

// HandleGetSubscription retrieves the current subscription details.
// GET /api/v1/billing/subscription (requires authentication)
func (h *Handlers) HandleGetSubscription(w http.ResponseWriter, r *http.Request) {
	p, ok := middleware.GetPrincipal(r.Context())
	if !ok {
		h.respondError(w, appErrors.NewUnauthorized("authentication required"))
		return
	}

	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		h.respondError(w, appErrors.NewInternal("invalid user ID", err))
		return
	}

	res, err := h.service.GetSubscription(r.Context(), userID)
	if err != nil {
		h.respondError(w, err)
		return
	}

	h.respond(w, http.StatusOK, res)
}

// HandleCreatePortalSession constructs a billing portal link.
// POST /api/v1/billing/portal (requires authentication)
func (h *Handlers) HandleCreatePortalSession(w http.ResponseWriter, r *http.Request) {
	p, ok := middleware.GetPrincipal(r.Context())
	if !ok {
		h.respondError(w, appErrors.NewUnauthorized("authentication required"))
		return
	}

	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		h.respondError(w, appErrors.NewInternal("invalid user ID", err))
		return
	}

	res, err := h.service.CreatePortalSession(r.Context(), userID)
	if err != nil {
		h.respondError(w, err)
		return
	}

	h.respond(w, http.StatusOK, res)
}

// HandleCancelSubscription schedules subscription cancellation.
// POST /api/v1/billing/cancel (requires authentication)
func (h *Handlers) HandleCancelSubscription(w http.ResponseWriter, r *http.Request) {
	p, ok := middleware.GetPrincipal(r.Context())
	if !ok {
		h.respondError(w, appErrors.NewUnauthorized("authentication required"))
		return
	}

	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		h.respondError(w, appErrors.NewInternal("invalid user ID", err))
		return
	}

	err = h.service.CancelSubscription(r.Context(), userID)
	if err != nil {
		h.respondError(w, err)
		return
	}

	h.respond(w, http.StatusOK, map[string]string{"message": "subscription cancellation scheduled successfully"})
}

// ---------------------------------------------------------------------------
// HTTP Helpers
// ---------------------------------------------------------------------------

func (h *Handlers) decode(r *http.Request, val any) error {
	if err := json.NewDecoder(r.Body).Decode(val); err != nil {
		if errors.Is(err, io.EOF) {
			return appErrors.NewBadRequest("request body is empty")
		}
		return appErrors.NewBadRequest("invalid JSON body")
	}
	return nil
}

func (h *Handlers) validateStruct(val any) error {
	if err := h.validate.Struct(val); err != nil {
		var validationErrors validator.ValidationErrors
		if errors.As(err, &validationErrors) {
			appErr := appErrors.NewValidation("validation failed")
			for _, fieldErr := range validationErrors {
				appErr = appErr.WithDetail(fieldErr.Field() + ": " + fieldErr.Tag())
			}
			return appErr
		}
		return appErrors.NewBadRequest("validation failed")
	}
	return nil
}

func (h *Handlers) respond(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("failed to encode response", "error", err)
	}
}
