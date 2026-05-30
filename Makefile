.PHONY: build run test lint migrate-up migrate-down migrate-create sqlc templ css dev docker-build docker-up docker-down generate clean seed setup check

# Build all binaries
build:
	go build -o bin/api ./cmd/api
	go build -o bin/worker ./cmd/worker
	go build -o bin/migrate ./cmd/migrate

# Run the API server
run:
	@if [ -f .env ]; then \
		export $$(grep -v '^#' .env | xargs) && go run ./cmd/api; \
	else \
		go run ./cmd/api; \
	fi

# Run tests with race detector
test:
	go test ./... -v -race

# Run linter
lint:
	golangci-lint run

# Database migrations
migrate-up:
	@if [ -f .env ]; then \
		export $$(grep -v '^#' .env | xargs) && go run ./cmd/migrate up; \
	else \
		go run ./cmd/migrate up; \
	fi

migrate-down:
	@if [ -f .env ]; then \
		export $$(grep -v '^#' .env | xargs) && go run ./cmd/migrate down; \
	else \
		go run ./cmd/migrate down; \
	fi

migrate-create:
	goose -dir migrations create $(name) sql

# Code generation
sqlc:
	sqlc generate

templ:
	templ generate

# Build Tailwind CSS
css:
	./tailwindcss-linux-x64 -i web/assets/input.css -o web/static/css/style.css --minify

# Development with hot reload
dev:
	@if command -v air > /dev/null 2>&1; then \
		air; \
	else \
		echo "air not found, using go run"; \
		go run ./cmd/api; \
	fi

# Docker targets
docker-build:
	docker compose -f deployments/docker-compose.yml build

docker-up:
	docker compose -f deployments/docker-compose.yml up -d

docker-down:
	docker compose -f deployments/docker-compose.yml down

# Run all code generators
generate: sqlc templ

# Clean build artifacts
clean:
	rm -rf bin/
	rm -rf tmp/
	rm -f web/static/css/style.css

# Seed test database users
seed:
	@if [ -f .env ]; then \
		export $$(grep -v '^#' .env | xargs) && go run ./cmd/seed/main.go; \
	else \
		go run ./cmd/seed/main.go; \
	fi

setup: docker-up migrate-up generate css seed

check:
	golangci-lint run
	go test ./... -race
	sqlc generate
	templ generate
	./tailwindcss-linux-x64 -i web/assets/input.css -o web/static/css/style.css --minify
