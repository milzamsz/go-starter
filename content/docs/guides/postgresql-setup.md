# PostgreSQL Setup

GoStarter uses PostgreSQL as the default persistence layer.

## Local setup

Use Docker Compose through:

```bash
make docker-up
```

## Migrations

Apply migrations:

```bash
make migrate-up
```

Rollback one step when needed:

```bash
make migrate-down
```

## Seed data

Create test accounts with:

```bash
make seed
```
