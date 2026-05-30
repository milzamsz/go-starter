package users

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"

	appErrors "github.com/milzam/go-starter/internal/errors"
	"github.com/milzam/go-starter/internal/middleware"
)

// Handlers defines the HTTP handlers for the users API.
type Handlers struct {
	service  *Service
	validate *validator.Validate
	logger   *slog.Logger
}

// NewHandlers creates a new Handlers instance with explicit dependencies.
func NewHandlers(service *Service, validate *validator.Validate, logger *slog.Logger) *Handlers {
	return &Handlers{
		service:  service,
		validate: validate,
		logger:   logger.With("component", "users.Handlers"),
	}
}

// HandleGetUser returns a single user's profile.
// GET /api/v1/users/{id}
func (h *Handlers) HandleGetUser(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	if idStr == "" {
		idStr = r.URL.Query().Get("id") // fallback
	}

	id, err := uuid.Parse(idStr)
	if err != nil {
		h.respondError(w, appErrors.NewBadRequest("invalid user ID format"))
		return
	}

	// Enforce that a regular user can only view their own profile.
	// Admin role is allowed to view any profile.
	p, ok := middleware.GetPrincipal(r.Context())
	if !ok {
		h.respondError(w, appErrors.NewUnauthorized("authentication required"))
		return
	}

	if p.Role != "admin" && p.UserID != id.String() {
		h.respondError(w, appErrors.NewForbidden("you do not have permission to view this profile"))
		return
	}

	user, err := h.service.GetUser(r.Context(), id)
	if err != nil {
		h.respondError(w, err)
		return
	}

	h.respond(w, http.StatusOK, user)
}

// HandleListUsers returns a paginated list of users.
// GET /api/v1/users (admin only)
func (h *Handlers) HandleListUsers(w http.ResponseWriter, r *http.Request) {
	// Parse pagination parameters.
	page := 1
	pageSize := 20

	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	if sizeStr := r.URL.Query().Get("page_size"); sizeStr != "" {
		if s, err := strconv.Atoi(sizeStr); err == nil && s > 0 {
			pageSize = s
		}
	}

	req := ListUsersRequest{
		Page:     page,
		PageSize: pageSize,
	}

	if err := h.validateStruct(req); err != nil {
		h.respondError(w, err)
		return
	}

	res, err := h.service.ListUsers(r.Context(), req.Page, req.PageSize)
	if err != nil {
		h.respondError(w, err)
		return
	}

	h.respond(w, http.StatusOK, res)
}

// HandleUpdateUser updates a user's name.
// PUT /api/v1/users/{id}
func (h *Handlers) HandleUpdateUser(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	if idStr == "" {
		idStr = r.URL.Query().Get("id")
	}

	id, err := uuid.Parse(idStr)
	if err != nil {
		h.respondError(w, appErrors.NewBadRequest("invalid user ID format"))
		return
	}

	// Enforce that a regular user can only update their own profile.
	p, ok := middleware.GetPrincipal(r.Context())
	if !ok {
		h.respondError(w, appErrors.NewUnauthorized("authentication required"))
		return
	}

	if p.Role != "admin" && p.UserID != id.String() {
		h.respondError(w, appErrors.NewForbidden("you do not have permission to update this profile"))
		return
	}

	var req UpdateUserRequest
	if err := h.decode(r, &req); err != nil {
		h.respondError(w, err)
		return
	}

	if err := h.validateStruct(req); err != nil {
		h.respondError(w, err)
		return
	}

	user, err := h.service.UpdateUser(r.Context(), id, req)
	if err != nil {
		h.respondError(w, err)
		return
	}

	h.respond(w, http.StatusOK, user)
}

// HandleUpdateUserRole modifies a user's role (admin only).
// PATCH /api/v1/users/{id}/role
func (h *Handlers) HandleUpdateUserRole(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	if idStr == "" {
		idStr = r.URL.Query().Get("id")
	}

	id, err := uuid.Parse(idStr)
	if err != nil {
		h.respondError(w, appErrors.NewBadRequest("invalid user ID format"))
		return
	}

	var req UpdateUserRoleRequest
	if err := h.decode(r, &req); err != nil {
		h.respondError(w, err)
		return
	}

	if err := h.validateStruct(req); err != nil {
		h.respondError(w, err)
		return
	}

	if err := h.service.UpdateUserRole(r.Context(), id, req.Role); err != nil {
		h.respondError(w, err)
		return
	}

	h.respond(w, http.StatusOK, map[string]string{"message": "user role updated successfully"})
}

// HandleDeleteUser deletes a user (admin only).
// DELETE /api/v1/users/{id}
func (h *Handlers) HandleDeleteUser(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	if idStr == "" {
		idStr = r.URL.Query().Get("id")
	}

	id, err := uuid.Parse(idStr)
	if err != nil {
		h.respondError(w, appErrors.NewBadRequest("invalid user ID format"))
		return
	}

	if err := h.service.DeleteUser(r.Context(), id); err != nil {
		h.respondError(w, err)
		return
	}

	h.respond(w, http.StatusOK, map[string]string{"message": "user deleted successfully"})
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

func (h *Handlers) respondError(w http.ResponseWriter, err error) {
	var appErr *appErrors.AppError
	if !errors.As(err, &appErr) {
		appErr = appErrors.NewInternal("an unexpected error occurred", err)
	}

	status := appErrors.HTTPStatus(appErr)
	h.respond(w, status, appErr)
}
