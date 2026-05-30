# Security

Security defaults are embedded into both API and web flows.

## Included protections

- CSRF validation for web form mutations
- role-gated admin routes
- CORS allowlist via environment configuration
- security headers including CSP and clickjacking protection
- secure cookie handling based on TLS/proxy context

## Recommended checks

Validate cookie flags, CORS origins, and webhook secret configuration in every environment.
