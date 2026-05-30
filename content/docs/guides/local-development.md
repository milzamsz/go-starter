# Local Development

Daily development flow in GoStarter:

## Start dependencies

```bash
make docker-up
make migrate-up
```

## Generate assets

```bash
make generate
make css
```

## Run services

```bash
make run
go run ./cmd/worker
```

## Validate before push

```bash
make check
```
