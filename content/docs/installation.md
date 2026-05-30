# Installation

This guide gets GoStarter running locally.

## Prerequisites

- Go `1.25+`
- Docker and Docker Compose
- `templ`, `sqlc`, `golangci-lint` installed locally

## Setup

```bash
cp .env.example .env
make setup
make run
```

## Local URLs

- App: `http://localhost:8080`
- Docs: `http://localhost:8080/docs`
- Showcase: `http://localhost:8080/showcase`

## Verify

Run:

```bash
make check
```
