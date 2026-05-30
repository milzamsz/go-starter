# Background Jobs

Background work runs through Asynq workers.

## Included tasks

- email delivery tasks
- token/session cleanup tasks
- billing follow-up tasks

## Runtime model

- API enqueues asynchronous work.
- Worker consumes queues and retries failures.

## Local development

Run `go run ./cmd/worker` alongside the API process.
