package auth

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/starfederation/datastar-go/datastar"

	"github.com/milzam/go-starter/internal/config"
	appErrors "github.com/milzam/go-starter/internal/errors"
	"github.com/milzam/go-starter/internal/middleware"
	"github.com/milzam/go-starter/web/templates/shared"
)

// Handlers defines the HTTP handlers for the authentication API.
type Handlers struct {
	service  *Service
	validate *validator.Validate
	logger   *slog.Logger
	cfg      config.AuthConfig
}

// NewHandlers creates a new Handlers instance with explicit dependencies.
func NewHandlers(service *Service, validate *validator.Validate, logger *slog.Logger, cfg config.AuthConfig) *Handlers {
	return &Handlers{
		service:  service,
		validate: validate,
		logger:   logger.With("component", "auth.Handlers"),
		cfg:      cfg,
	}
}

// HandleSignup processes a registration request.
// POST /api/v1/auth/signup
func (h *Handlers) HandleSignup(w http.ResponseWriter, r *http.Request) {
	var req SignupRequest
	if err := h.decode(r, &req); err != nil {
		if strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
			sse := datastar.NewSSE(w, r)
			_ = sse.PatchElementTempl(shared.ErrorAlert(err.Error()))
			return
		}
		h.respondError(w, err)
		return
	}

	if err := h.validateStruct(req); err != nil {
		if strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
			sse := datastar.NewSSE(w, r)
			_ = sse.PatchElementTempl(shared.ErrorAlert(err.Error()))
			return
		}
		h.respondError(w, err)
		return
	}

	res, err := h.service.Signup(r.Context(), req)
	if err != nil {
		if strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
			sse := datastar.NewSSE(w, r)
			_ = sse.PatchElementTempl(shared.ErrorAlert(err.Error()))
			return
		}
		h.respondError(w, err)
		return
	}

	h.setSessionCookie(w, r, res.AccessToken)

	if strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
		sse := datastar.NewSSE(w, r)
		_ = sse.Redirect("/dashboard")
		return
	}

	h.respond(w, http.StatusCreated, res)
}

// HandleLogin processes a login request.
// POST /api/v1/auth/login
func (h *Handlers) HandleLogin(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := h.decode(r, &req); err != nil {
		if strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
			sse := datastar.NewSSE(w, r)
			_ = sse.PatchElementTempl(shared.ErrorAlert(err.Error()))
			return
		}
		h.respondError(w, err)
		return
	}

	if err := h.validateStruct(req); err != nil {
		if strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
			sse := datastar.NewSSE(w, r)
			_ = sse.PatchElementTempl(shared.ErrorAlert(err.Error()))
			return
		}
		h.respondError(w, err)
		return
	}

	res, err := h.service.Login(r.Context(), req)
	if err != nil {
		if strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
			sse := datastar.NewSSE(w, r)
			_ = sse.PatchElementTempl(shared.ErrorAlert(err.Error()))
			return
		}
		h.respondError(w, err)
		return
	}

	h.setSessionCookie(w, r, res.AccessToken)

	if strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
		sse := datastar.NewSSE(w, r)
		_ = sse.Redirect("/dashboard")
		return
	}

	h.respond(w, http.StatusOK, res)
}

// HandleLogout clears the session cookie.
// POST /api/v1/auth/logout
func (h *Handlers) HandleLogout(w http.ResponseWriter, r *http.Request) {
	h.clearSessionCookie(w, r)

	// Check if this is a Datastar SSE request
	if strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
		sse := datastar.NewSSE(w, r)
		_ = sse.Redirect("/login")
		return
	}

	h.respond(w, http.StatusOK, map[string]string{"message": "logged out successfully"})
}

// HandleRefresh handles access token renewal via refresh token.
// POST /api/v1/auth/refresh
func (h *Handlers) HandleRefresh(w http.ResponseWriter, r *http.Request) {
	var req TokenRequest
	if err := h.decode(r, &req); err != nil {
		h.respondError(w, err)
		return
	}

	if err := h.validateStruct(req); err != nil {
		h.respondError(w, err)
		return
	}

	res, err := h.service.RefreshToken(r.Context(), req.Token)
	if err != nil {
		h.respondError(w, err)
		return
	}

	h.setSessionCookie(w, r, res.AccessToken)
	h.respond(w, http.StatusOK, res)
}

// HandleVerifyEmail verifies a user's email with a token.
// POST /api/v1/auth/verify-email
func (h *Handlers) HandleVerifyEmail(w http.ResponseWriter, r *http.Request) {
	var req TokenRequest
	if err := h.decode(r, &req); err != nil {
		h.respondError(w, err)
		return
	}

	if err := h.validateStruct(req); err != nil {
		h.respondError(w, err)
		return
	}

	if err := h.service.VerifyEmail(r.Context(), req.Token); err != nil {
		h.respondError(w, err)
		return
	}

	h.respond(w, http.StatusOK, map[string]string{"message": "email verified successfully"})
}

// HandleForgotPassword initiates a password reset flow.
// POST /api/v1/auth/forgot-password
func (h *Handlers) HandleForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req ForgotPasswordRequest
	if err := h.decode(r, &req); err != nil {
		h.respondError(w, err)
		return
	}

	if err := h.validateStruct(req); err != nil {
		h.respondError(w, err)
		return
	}

	if err := h.service.ForgotPassword(r.Context(), req.Email); err != nil {
		h.respondError(w, err)
		return
	}

	// Always respond with a generic success message to prevent user enumeration.
	h.respond(w, http.StatusOK, map[string]string{"message": "password reset email sent if account exists"})
}

// HandleResetPassword completes the password reset flow.
// POST /api/v1/auth/reset-password
func (h *Handlers) HandleResetPassword(w http.ResponseWriter, r *http.Request) {
	var req ResetPasswordRequest
	if err := h.decode(r, &req); err != nil {
		h.respondError(w, err)
		return
	}

	if err := h.validateStruct(req); err != nil {
		h.respondError(w, err)
		return
	}

	if err := h.service.ResetPassword(r.Context(), req); err != nil {
		h.respondError(w, err)
		return
	}

	h.respond(w, http.StatusOK, map[string]string{"message": "password reset successfully"})
}

// HandleGetMe returns the authenticated user's profile.
// GET /api/v1/auth/me (requires authentication)
func (h *Handlers) HandleGetMe(w http.ResponseWriter, r *http.Request) {
	p, ok := middleware.GetPrincipal(r.Context())
	if !ok {
		h.respondError(w, appErrors.NewUnauthorized("authentication required"))
		return
	}

	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "parsing principal user ID", "error", err)
		h.respondError(w, appErrors.NewInternal("invalid principal user ID", err))
		return
	}

	info, err := h.service.GetCurrentUser(r.Context(), userID)
	if err != nil {
		h.respondError(w, err)
		return
	}

	h.respond(w, http.StatusOK, info)
}

// HandleChangePassword updates the authenticated user's password.
// POST /api/v1/auth/change-password (requires authentication)
func (h *Handlers) HandleChangePassword(w http.ResponseWriter, r *http.Request) {
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

	var req ChangePasswordRequest
	if err := h.decode(r, &req); err != nil {
		h.respondError(w, err)
		return
	}

	if err := h.validateStruct(req); err != nil {
		h.respondError(w, err)
		return
	}

	if err := h.service.ChangePassword(r.Context(), userID, req); err != nil {
		h.respondError(w, err)
		return
	}

	h.respond(w, http.StatusOK, map[string]string{"message": "password updated successfully"})
}

// HandleEnable2FA initiates 2FA setup.
// POST /api/v1/auth/2fa/enable (requires authentication)
func (h *Handlers) HandleEnable2FA(w http.ResponseWriter, r *http.Request) {
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

	res, err := h.service.Enable2FA(r.Context(), userID)
	if err != nil {
		h.respondError(w, err)
		return
	}

	h.respond(w, http.StatusOK, res)
}

// HandleVerify2FA completes 2FA setup.
// POST /api/v1/auth/2fa/verify (requires authentication)
func (h *Handlers) HandleVerify2FA(w http.ResponseWriter, r *http.Request) {
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

	var req Verify2FARequest
	if err := h.decode(r, &req); err != nil {
		h.respondError(w, err)
		return
	}

	if err := h.validateStruct(req); err != nil {
		h.respondError(w, err)
		return
	}

	if err := h.service.Verify2FA(r.Context(), userID, req.Code); err != nil {
		h.respondError(w, err)
		return
	}

	h.respond(w, http.StatusOK, map[string]string{"message": "2FA setup verified and enabled successfully"})
}

// HandleDisable2FA disables 2FA.
// POST /api/v1/auth/2fa/disable (requires authentication)
func (h *Handlers) HandleDisable2FA(w http.ResponseWriter, r *http.Request) {
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

	if err := h.service.Disable2FA(r.Context(), userID); err != nil {
		h.respondError(w, err)
		return
	}

	h.respond(w, http.StatusOK, map[string]string{"message": "2FA disabled successfully"})
}

// HandleOAuthCallback handles the OAuth redirect callback.
// GET /auth/callback/{provider}
func (h *Handlers) HandleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	provider := r.PathValue("provider")
	if provider == "" {
		// Chi route patterns in standard library net/http or go-chi might place this differently.
		// Fallback path value extraction if using chi:
		provider = r.URL.Query().Get("provider")
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		h.respondError(w, appErrors.NewBadRequest("auth code is required"))
		return
	}

	res, err := h.service.OAuthLogin(r.Context(), provider, code)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "OAuth login failed", "provider", provider, "error", err)
		// Redirect to login page with an error query param
		http.Redirect(w, r, "/login?error=oauth_failed", http.StatusTemporaryRedirect)
		return
	}

	h.setSessionCookie(w, r, res.AccessToken)
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
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

func (h *Handlers) setSessionCookie(w http.ResponseWriter, r *http.Request, token string) {
	// Dynamically set Secure flag based on TLS connection or reverse proxy headers
	secure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"

	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    token,
		Path:     "/",
		MaxAge:   h.cfg.SessionMaxAge,
		Secure:   secure,
		HttpOnly: true, // prevent access via client-side scripts
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *Handlers) clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	secure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		Secure:   secure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
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
