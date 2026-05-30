package server

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/milzam/go-starter/internal/app"
	"github.com/milzam/go-starter/internal/middleware"
)

// NewServer creates and configures the HTTP server with all routes.
func NewServer(application *app.App) *http.Server {
	r := chi.NewRouter()

	// Global middleware stack (order matters)
	r.Use(chiMiddleware.RealIP)
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger(application.Logger))
	r.Use(chiMiddleware.Recoverer)
	r.Use(middleware.SecurityHeaders)
	r.Use(middleware.EnsureCSRFCookie)
	r.Use(middleware.CSRFMiddleware)
	r.Use(middleware.RateLimiter(100, 100)) // 100 req/s burst
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   application.Config.App.AllowedOrigins(),
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	// Register all routes
	RegisterRoutes(r, application)

	return &http.Server{
		Addr:         application.Config.HTTP.Addr(),
		Handler:      r,
		ReadTimeout:  application.Config.HTTP.ReadTimeout,
		WriteTimeout: application.Config.HTTP.WriteTimeout,
		IdleTimeout:  application.Config.HTTP.IdleTimeout,
	}
}

// NewDevServer creates a development server with relaxed timeouts.
func NewDevServer(application *app.App) *http.Server {
	srv := NewServer(application)
	srv.ReadTimeout = 30 * time.Second
	srv.WriteTimeout = 30 * time.Second
	return srv
}
