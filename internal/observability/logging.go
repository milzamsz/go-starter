package observability

import (
	"log/slog"
	"os"
)

// SetupLogger creates a configured *slog.Logger based on the application environment.
// In production, it outputs JSON at Info level for structured log aggregation.
// In development (or any other env), it outputs human-readable text at Debug level
// with source file locations enabled.
func SetupLogger(env string) *slog.Logger {
	var handler slog.Handler

	switch env {
	case "production":
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})
	default:
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level:     slog.LevelDebug,
			AddSource: true,
		})
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)

	return logger
}
