# AGENTS.md

Guidance for AI coding agents (and new contributors) working in **go-starter**, a production-ready Go SaaS boilerplate. Read this before making changes.

## What this project is

A single-tenant Go web application and JSON API with server-rendered UI. It bundles authentication, RBAC authorization, Stripe billing, background jobs, and a component-driven frontend. Module path: `github.com/milzam/go-starter`. Go version: **1.25+**.

## Tech stack

| Concern | Choice |
| --- | --- |
| HTTP router | `chi/v5` |
| Database | PostgreSQL via `pgx/v5` connection pool |
| Query layer | `sqlc` (generated, type-safe) — never hand-write `database/sql` |
| Migrations | `goose` (SQL files in `migrations/`) |
| HTML templates | `templ` (compiled to Go) |
| Frontend interactivity | Datastar + templui components + Tailwind CSS |
| Auth | JWT (access/refresh) + server-side session cookie + OAuth (Google/GitHub) + TOTP 2FA |
| Authorization | Casbin RBAC (`internal/authz`) |
| Billing | Stripe (`stripe-go/v81`) with webhook idempotency |
| Background jobs | Asynq (Redis-backed) |
| Config | `envconfig` from environment variables |
| Logging | `slog` structured logging |

## Architecture & conventions

**Explicit dependency injection, no globals.** The central `app.App` struct (`internal/app/app.go`) holds all shared dependencies (config, DB pool, logger, queries, services, enforcer, queue client). It is constructed once in `cmd/api/main.go` via `app.New(...)` and threaded through to route registration. Do not introduce package-level singletons or `init()`-based wiring.

**Layering.** Roughly: handlers (HTTP concerns, validation, response shaping) → services (`internal/modules/*/service.go`, business logic) → `sqlc.Queries` (data access). Keep business logic out of handlers and SQL out of services (use generated queries).

**Feature modules** live under `internal/modules/<name>/` and follow a consistent file layout:

- `types.go` — request/response DTOs
- `service.go` — business logic, constructed with `NewService(...)`
- `handlers.go` — HTTP handlers, constructed with `NewHandlers(...)`
- `webhook.go` — (billing only) Stripe webhook handling

Existing modules: `users`, `billing`. `auth` lives at `internal/auth/` (slightly older layout but same idea: `service.go`, `handlers_api.go`, `jwt.go`, `oauth.go`, `totp.go`, `types.go`).

**Entry points** (`cmd/`):

- `cmd/api` — the HTTP server
- `cmd/worker` — the Asynq background job processor
- `cmd/migrate` — runs goose migrations (`up`/`down`)
- `cmd/seed` — seeds test users

**Routing** is centralized in `internal/server/routes.go` (`RegisterRoutes`). The global middleware stack is in `internal/server/server.go` and **order matters**: RealIP → RequestID → Logger → Recoverer → SecurityHeaders → CSRF → RateLimiter → CORS. There are two parallel surfaces: a JSON API under `/api/v1/*` and server-rendered web pages (templ). Web form posts (e.g. `*-web` routes) internally re-dispatch to API handlers via a lightweight `responseRecorder`.

## Critical rule: code generation

Several directories are **generated — never edit by hand**:

- `internal/sqlc/*.go` — regenerate with `make sqlc` (config in `sqlc.yaml`). To change queries, edit `sql/queries/*.sql` and the schema in `migrations/`, then regenerate.
- `web/templates/**/*_templ.go` — regenerate with `make templ`. Edit the `.templ` source files instead.
- `web/static/css/style.css` — regenerate with `make css`.

Run `make generate` (sqlc + templ) after touching SQL or templates. `make check` regenerates everything and is drift-sensitive — if generated output changes, you forgot to regenerate.

## Database

Schema is defined in goose migration files under `migrations/` (`00001_init.sql` is the base schema: `users`, `oauth_identities`, `sessions`, `auth_tokens`, `webhook_events`, plus enums and an `updated_at` trigger). Billing fields are denormalized onto `users` from Stripe.

Create a migration with `make migrate-create name=<desc>`, write both `+goose Up` and `+goose Down`, then update queries in `sql/queries/` and run `make sqlc`. UUIDs map to `github.com/google/uuid`, `timestamptz` to `time.Time`, nullable columns to pointers (`emit_pointers_for_null_types: true`).

## Background jobs

Task type constants live in `internal/queue/tasks.go`. The producer enqueues via the `queue.Client`; the consumer (`cmd/worker`) registers handlers from the `tasks/` package on an Asynq `ServeMux`. To add a job: define a constant in `queue/tasks.go`, add a handler in `tasks/`, register it in `cmd/worker/main.go`, and enqueue it from the relevant service. The API and worker are separate processes — both must be running for end-to-end job flow.

## Authorization

Casbin RBAC. The model is `internal/authz/model.conf`; default in-memory policies are in `internal/authz/enforcer.go` (`defaultPolicies`). Roles are `user` and `admin` (DB enum `user_role`). Admin-gated routes use `authz.Authorize(enforcer, logger)` middleware after `RequireAuth`. Policies are in-memory by default — for production, swap the string adapter for a DB-backed adapter (noted in code comments).

## Configuration

All config comes from environment variables, parsed into typed structs in `internal/config/config.go`. `JWT_SECRET` and `SESSION_SECRET` are **required**. Copy `.env.example` to `.env` for local dev. `make run` auto-exports `.env`. Do not commit secrets.

## Developer workflow

```bash
make setup        # docker-up + migrate-up + generate + css + seed (first-time bootstrap)
make run          # run the API server (loads .env)
make dev          # hot reload via air (falls back to go run)
make generate     # sqlc + templ
make css          # build Tailwind
make test         # go test ./... -race
make lint         # golangci-lint
make check        # lint + race tests + regenerate (run before committing)
make build        # build api, worker, migrate binaries into bin/
```

Run the worker separately during development if exercising jobs: `go run ./cmd/worker`.

## Conventions to follow

- Wrap errors with context: `fmt.Errorf("doing X: %w", err)`.
- Use `slog` for logging; the logger is injected, not global (though `slog.SetDefault` is set at startup).
- Validate request DTOs with `go-playground/validator` (a shared `validate` instance is created in `RegisterRoutes`).
- Keep the Stripe webhook route (`/webhooks/stripe`) free of auth/CSRF middleware — it reads the raw body and verifies the Stripe signature. Webhook idempotency is enforced via the `webhook_events` table.
- Session cookie `Secure` flag follows TLS / `X-Forwarded-Proto`.
- CORS origins come from `CORS_ALLOWED_ORIGINS`.

## Before you finish a change

1. `make generate` if you touched SQL or `.templ` files.
2. `make check` — must pass with no generated-file drift.
3. Add/adjust tests (`*_test.go`) for new logic; tests run with `-race`.
4. Keep changes within the established module/layer boundaries.
