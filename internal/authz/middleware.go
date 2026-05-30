package authz

import (
	"log/slog"
	"net/http"

	"github.com/milzam/go-starter/internal/middleware"
)

// Authorize returns middleware that checks if the authenticated user's role
// has permission for the requested resource and action via the Casbin enforcer.
//
// It expects AuthContext middleware to have already run and placed a Principal
// in the request context. If no principal is found, it returns 401. If the
// policy denies access, it returns 403.
func Authorize(enforcer *Enforcer, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, ok := middleware.GetPrincipal(r.Context())
			if !ok {
				writeJSONError(w, http.StatusUnauthorized,
					`{"error":"unauthorized","message":"authentication required"}`)
				return
			}

			allowed, err := enforcer.Enforce(principal.Role, r.URL.Path, r.Method)
			if err != nil {
				logger.ErrorContext(r.Context(), "authz enforcement error",
					slog.String("role", principal.Role),
					slog.String("resource", r.URL.Path),
					slog.String("action", r.Method),
					slog.String("error", err.Error()),
				)
				writeJSONError(w, http.StatusInternalServerError,
					`{"error":"internal","message":"authorization check failed"}`)
				return
			}

			if !allowed {
				logger.WarnContext(r.Context(), "authz denied",
					slog.String("user_id", principal.UserID),
					slog.String("role", principal.Role),
					slog.String("resource", r.URL.Path),
					slog.String("action", r.Method),
				)
				writeJSONError(w, http.StatusForbidden,
					`{"error":"forbidden","message":"insufficient permissions"}`)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// writeJSONError writes a JSON error response.
func writeJSONError(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}
