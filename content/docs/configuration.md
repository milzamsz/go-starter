# Configuration

GoStarter reads environment settings from `.env` (or deployment environment variables).

## Core application

- `APP_ENV`
- `APP_PORT`
- `APP_URL`
- `CORS_ALLOWED_ORIGINS`

## Database and cache

- `DATABASE_URL`
- `REDIS_ADDR`

## Authentication

- `JWT_SECRET`
- session cookie behavior follows request TLS / `X-Forwarded-Proto`

## Billing

- `STRIPE_SECRET_KEY`
- `STRIPE_WEBHOOK_SECRET`
- `STRIPE_PRICE_ID_PRO_MONTHLY` or default price id variable used by your app

## Email and storage

- SMTP variables for transactional email
- storage driver variables for local/object storage modes

## Recommended flow

Start from `.env.example`, then adjust Stripe, CORS, and app URL values before production deployment.
