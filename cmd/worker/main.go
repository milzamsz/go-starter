package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hibiken/asynq"
	"github.com/milzam/go-starter/internal/config"
	"github.com/milzam/go-starter/internal/database"
	"github.com/milzam/go-starter/internal/email"
	"github.com/milzam/go-starter/internal/observability"
	"github.com/milzam/go-starter/internal/queue"
	"github.com/milzam/go-starter/tasks"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("worker failed: %v", err)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	logger := observability.SetupLogger(cfg.App.Env)
	slog.SetDefault(logger)

	srv := queue.NewServer(
		cfg.Redis.Addr,
		cfg.Redis.Password,
		cfg.Redis.DB,
		cfg.Queue.Concurrency,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := database.NewPool(ctx, cfg.DB)
	if err != nil {
		return err
	}
	defer db.Close()

	emailSender := email.NewSMTPSender(cfg.Email)
	emailHandler := tasks.NewEmailHandler(logger, emailSender)
	cleanupHandler := tasks.NewCleanupHandler(db, logger)
	billingHandler := tasks.NewBillingHandler(logger)

	mux := asynq.NewServeMux()
	mux.Use(loggingMiddleware(logger))
	mux.HandleFunc(queue.TaskSendVerificationEmail, emailHandler.HandleSendVerificationEmail)
	mux.HandleFunc(queue.TaskSendResetEmail, emailHandler.HandleSendResetEmail)
	mux.HandleFunc(queue.TaskSendWelcomeEmail, emailHandler.HandleSendWelcomeEmail)
	mux.HandleFunc(queue.TaskCleanupExpiredTokens, cleanupHandler.HandleCleanupExpiredTokens)
	mux.HandleFunc(queue.TaskCleanupExpiredSessions, cleanupHandler.HandleCleanupExpiredSessions)
	mux.HandleFunc(queue.TaskProcessStripeWebhook, billingHandler.HandleProcessStripeWebhook)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-quit
		logger.Info("shutting down worker", slog.String("signal", sig.String()))
		srv.Shutdown()
	}()

	logger.Info("starting worker",
		slog.String("redis", cfg.Redis.Addr),
		slog.Int("concurrency", cfg.Queue.Concurrency),
	)

	if err := srv.Run(mux); err != nil {
		return err
	}

	logger.Info("worker stopped gracefully")
	return nil
}

func loggingMiddleware(logger *slog.Logger) asynq.MiddlewareFunc {
	return func(h asynq.Handler) asynq.Handler {
		return asynq.HandlerFunc(func(ctx context.Context, t *asynq.Task) error {
			logger.Info("processing task",
				slog.String("type", t.Type()),
				slog.Int("payload_size", len(t.Payload())),
			)
			err := h.ProcessTask(ctx, t)
			if err != nil {
				logger.Error("task failed",
					slog.String("type", t.Type()),
					slog.String("error", err.Error()),
				)
				return err
			}
			logger.Info("task completed", slog.String("type", t.Type()))
			return nil
		})
	}
}
