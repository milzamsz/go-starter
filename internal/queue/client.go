package queue

import "github.com/hibiken/asynq"

// Client wraps asynq.Client for task enqueueing.
type Client struct {
	client *asynq.Client
}

// NewClient creates a new queue Client connected to the given Redis instance.
func NewClient(redisAddr, redisPassword string, redisDB int) *Client {
	return &Client{
		client: asynq.NewClient(asynq.RedisClientOpt{
			Addr:     redisAddr,
			Password: redisPassword,
			DB:       redisDB,
		}),
	}
}

// Enqueue adds a task to the queue with the given options.
func (c *Client) Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	return c.client.Enqueue(task, opts...)
}

// Close releases the underlying Redis connection.
func (c *Client) Close() error {
	return c.client.Close()
}
