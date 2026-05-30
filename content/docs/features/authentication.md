# Authentication

GoStarter ships with web and API authentication patterns.

## Included capabilities

- Signup/login/logout flows
- JWT access and refresh tokens for API clients
- Session cookie for web app navigation
- Password reset and email verification flows
- OAuth callback wiring
- TOTP 2FA endpoints

## Route surface

- API under `/api/v1/auth/*`
- Web pages under `/login`, `/signup`, `/forgot-password`, `/reset-password`, and `/verify-email`

## Role-aware behavior

Principal role is refreshed from the database to avoid stale role claims in long-lived sessions.
