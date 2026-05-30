package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/milzam/go-starter/internal/sqlc"
)

type fakeUserLookup struct {
	user sqlc.User
	err  error
}

func (f fakeUserLookup) GetUserByID(ctx context.Context, id uuid.UUID) (sqlc.User, error) {
	return f.user, f.err
}

func signedToken(t *testing.T, secret string, userID uuid.UUID, role string) string {
	t.Helper()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
		Email: "stale@example.com",
		Role:  role,
	})

	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("SignedString() error = %v", err)
	}

	return signed
}

func TestAuthContextRefreshesRoleFromDatabase(t *testing.T) {
	userID := uuid.New()
	token := signedToken(t, "secret", userID, "user")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "session_token", Value: token})
	rec := httptest.NewRecorder()

	var principal *Principal
	handler := AuthContext("secret", fakeUserLookup{
		user: sqlc.User{
			ID:    userID,
			Email: "admin@test.com",
			Role:  sqlc.UserRoleAdmin,
		},
	}, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, _ = GetPrincipal(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(rec, req)

	if principal == nil {
		t.Fatal("expected principal in context")
	}
	if principal.Role != string(sqlc.UserRoleAdmin) {
		t.Fatalf("principal role = %q, want %q", principal.Role, sqlc.UserRoleAdmin)
	}
	if principal.Email != "admin@test.com" {
		t.Fatalf("principal email = %q, want admin@test.com", principal.Email)
	}
}

func TestAuthContextDropsPrincipalWhenUserLookupFails(t *testing.T) {
	userID := uuid.New()
	token := signedToken(t, "secret", userID, "admin")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "session_token", Value: token})
	rec := httptest.NewRecorder()

	var hasPrincipal bool
	handler := AuthContext("secret", fakeUserLookup{err: context.Canceled}, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, hasPrincipal = GetPrincipal(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(rec, req)

	if hasPrincipal {
		t.Fatal("expected no principal when user lookup fails")
	}
}
