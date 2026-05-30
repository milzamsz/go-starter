package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/milzam/go-starter/internal/sqlc"
)

// authContextKey is an unexported type for context keys in this package.
type authContextKey string

const principalKey authContextKey = "principal"

// Principal holds the authenticated user's identity and role.
type Principal struct {
	UserID    string
	Email     string
	Role      string
	SessionID string
}

// Claims extends jwt.RegisteredClaims with application-specific fields.
type Claims struct {
	jwt.RegisteredClaims
	Email     string `json:"email"`
	Role      string `json:"role"`
	SessionID string `json:"session_id"`
}

// UserLookup resolves users by ID so request auth can refresh role data from
// the database instead of trusting stale token claims.
type UserLookup interface {
	GetUserByID(ctx context.Context, id uuid.UUID) (sqlc.User, error)
}

// AuthContext extracts auth info from a JWT Bearer token or a session_token
// cookie and puts a Principal into the request context.
//
// It does not reject unauthenticated requests; downstream middleware
// (RequireAuth, RequireRole) is responsible for access control.
func AuthContext(jwtSecret string, users UserLookup, logger *slog.Logger) func(http.Handler) http.Handler {
	keyFunc := func(_ *jwt.Token) (any, error) {
		return []byte(jwtSecret), nil
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenString := extractToken(r)
			if tokenString == "" {
				next.ServeHTTP(w, r)
				return
			}

			claims := &Claims{}
			token, err := jwt.ParseWithClaims(tokenString, claims, keyFunc,
				jwt.WithValidMethods([]string{"HS256", "HS384", "HS512"}),
			)
			if err != nil || !token.Valid {
				next.ServeHTTP(w, r)
				return
			}

			userID, err := uuid.Parse(claims.Subject)
			if err != nil {
				if logger != nil {
					logger.WarnContext(r.Context(), "auth context rejected invalid subject in token", slog.String("subject", claims.Subject))
				}
				next.ServeHTTP(w, r)
				return
			}

			user, err := users.GetUserByID(r.Context(), userID)
			if err != nil {
				if logger != nil {
					logger.WarnContext(r.Context(), "auth context failed to refresh user from database",
						slog.String("user_id", claims.Subject),
						slog.String("error", err.Error()),
					)
				}
				next.ServeHTTP(w, r)
				return
			}

			p := &Principal{
				UserID:    claims.Subject,
				Email:     user.Email,
				Role:      string(user.Role),
				SessionID: claims.SessionID,
			}

			ctx := context.WithValue(r.Context(), principalKey, p)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireAuth rejects requests that do not have a valid Principal in context.
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := GetPrincipal(r.Context()); !ok {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				writeJSON(w, http.StatusUnauthorized, `{"error":"unauthorized","message":"authentication required"}`)
				return
			}
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireRole rejects requests where the principal's role is not in the
// allowed set.
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		allowed[r] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, ok := GetPrincipal(r.Context())
			if !ok {
				writeJSON(w, http.StatusUnauthorized, `{"error":"unauthorized","message":"authentication required"}`)
				return
			}

			if _, match := allowed[p.Role]; !match {
				writeJSON(w, http.StatusForbidden, `{"error":"forbidden","message":"insufficient permissions"}`)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// GetPrincipal extracts the Principal from context. Returns nil, false when
// the context does not carry a principal.
func GetPrincipal(ctx context.Context) (*Principal, bool) {
	p, ok := ctx.Value(principalKey).(*Principal)
	return p, ok
}

// extractToken retrieves the JWT string from the Authorization header
// (Bearer scheme) or the session_token cookie.
func extractToken(r *http.Request) string {
	if auth := r.Header.Get("Authorization"); auth != "" {
		parts := strings.SplitN(auth, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			return strings.TrimSpace(parts[1])
		}
	}

	if cookie, err := r.Cookie("session_token"); err == nil {
		return cookie.Value
	}

	return ""
}

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}
