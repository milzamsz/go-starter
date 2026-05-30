# Architecture

GoStarter keeps boundaries explicit.

## Application layers

- `cmd/*`: application entry points (`api`, `worker`, `migrate`, `seed`)
- `internal/server`: HTTP router and page/API route wiring
- `internal/modules/*`: business use-cases such as billing and users
- `internal/sqlc`: generated query interfaces
- `tasks`: Asynq background handlers
- `web/templates`: templ-based UI pages and layouts

## Request flow

HTTP requests enter `chi`, pass middleware (auth context, security, CSRF, logging), then route to API handlers or templ pages.

## Data flow

Services call `sqlc` queries for data persistence and enqueue long-running work to Asynq workers.

## Why this shape

The goal is maintainability: predictable dependencies, typed SQL, and clear ownership across backend and UI.
