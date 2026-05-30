package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/milzam/go-starter/internal/app"
	"github.com/milzam/go-starter/internal/auth"
	"github.com/milzam/go-starter/internal/authz"
	"github.com/milzam/go-starter/internal/config"
	"github.com/milzam/go-starter/internal/database"
	"github.com/milzam/go-starter/internal/modules/billing"
	"github.com/milzam/go-starter/internal/modules/users"
	"github.com/milzam/go-starter/internal/observability"
	"github.com/milzam/go-starter/internal/queue"
	"github.com/milzam/go-starter/internal/server"
	"github.com/milzam/go-starter/internal/sqlc"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("application failed: %v", err)
	}
}

func run() error {
	// Explicitly register MIME types to fix Go-on-Windows registry issue
	_ = mime.AddExtensionType(".js", "application/javascript")
	_ = mime.AddExtensionType(".css", "text/css")

	// 1. Load configuration.
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// 2. Setup structured logger.
	logger := observability.SetupLogger(cfg.App.Env)
	slog.SetDefault(logger)

	// 3. Create database connection pool.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := database.NewPool(ctx, cfg.DB)
	if err != nil {
		return err
	}
	defer pool.Close()

	// 4. Initialize shared dependencies and services.
	queries := sqlc.New(pool)

	jwtManager := auth.NewJWTManager(cfg.Auth.JWTSecret, cfg.Auth.AccessExpiry, cfg.Auth.RefreshExpiry)
	queueClient := queue.NewClient(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
	authService := auth.NewService(queries, pool, jwtManager, logger, cfg.Auth, queueClient)

	// Set up OAuth providers
	if cfg.OAuth.GoogleClientID != "" {
		googleProvider := auth.NewGoogleProvider(cfg.OAuth.GoogleClientID, cfg.OAuth.GoogleClientSecret, cfg.OAuth.GoogleRedirectURL)
		authService.AddOAuthProvider(googleProvider)
	}
	if cfg.OAuth.GithubClientID != "" {
		githubProvider := auth.NewGithubProvider(cfg.OAuth.GithubClientID, cfg.OAuth.GithubClientSecret, cfg.OAuth.GithubRedirectURL)
		authService.AddOAuthProvider(githubProvider)
	}

	usersService := users.NewService(queries, pool, logger)
	billingService := billing.NewService(queries, pool, logger, cfg.Stripe)

	enforcer, err := authz.NewEnforcer()
	if err != nil {
		logger.Error("failed to construct casbin enforcer", "error", err)
		return err
	}

	// 5. Construct the central App struct.
	application := app.New(cfg, pool, logger, queries, authService, usersService, billingService, enforcer, queueClient)
	defer application.Close()

	// 6. Create HTTP server.
	srv := server.NewServer(application)

	// 7. Graceful shutdown.
	shutdownErr := make(chan error, 1)

	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		sig := <-quit

		logger.Info("shutting down server", slog.String("signal", sig.String()))

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		shutdownErr <- srv.Shutdown(ctx)
	}()

	// 8. Log startup info and start serving.
	logger.Info("starting server",
		slog.String("addr", cfg.HTTP.Addr()),
		slog.String("env", cfg.App.Env),
	)

	err = srv.ListenAndServe()
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	err = <-shutdownErr
	if err != nil {
		return err
	}

	logger.Info("server stopped gracefully")
	return nil
}
