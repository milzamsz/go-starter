# Updating GoStarter

Use a regeneration-first workflow when updating the starter.

## Regenerate artifacts

```bash
make generate
make css
```

## Validate behavior

```bash
go test ./...
make check
```

## Review drift

Confirm generated files are intentional before merging updates.

## Keep docs in sync

Update markdown docs for any route, config, or workflow changes.
