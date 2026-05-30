package queue

import (
	"context"
	"log/slog"

	"github.com/hibiken/asynq"
)

// NewServer creates a new asynq.Server configured with the given Redis
// connection parameters and concurrency level.
func NewServer(redisAddr, redisPassword string, redisDB, concurrency int) *asynq.Server {
	if concurrency <= 0 {
		concurrency = 10
	}

	return asynq.NewServer(
		asynq.RedisClientOpt{
			Addr:     redisAddr,
			Password: redisPassword,
			DB:       redisDB,
		},
		asynq.Config{
			Concurrency: concurrency,
			Queues: map[string]int{
				"critical": 6,
				"default":  3,
				"low":      1,
			},
			RetryDelayFunc: asynq.DefaultRetryDelayFunc,
			ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
				slog.Error("task processing error",
					slog.String("type", task.Type()),
					slog.String("error", err.Error()),
				)
			}),
		},
	)
}
