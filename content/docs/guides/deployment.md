# Deployment

This guide covers practical deployment steps for GoStarter.

## Environment preparation

- Set production `APP_URL`.
- Set strict `CORS_ALLOWED_ORIGINS`.
- Set strong `JWT_SECRET`.
- Configure Stripe and SMTP secrets.

## Services

Run:

- API service (`cmd/api`)
- Worker service (`cmd/worker`)
- Postgres
- Redis

## Webhooks

Stripe webhook endpoints must target your deployed `/webhooks/stripe` URL with the correct webhook secret.

## Validation before release

Run `make check` and confirm migrations, templates, and CSS are generated and committed.
