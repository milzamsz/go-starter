# ============================================================================
# Stage 1: Build
# ============================================================================
FROM golang:1.25-alpine AS build

RUN apk add --no-cache git ca-certificates

# Install templ CLI
RUN go install github.com/a-h/templ/cmd/templ@latest

WORKDIR /src

# Cache dependency downloads
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Generate templ files
RUN templ generate

# Build binaries with static linking
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/server ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/worker ./cmd/worker
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/migrate ./cmd/migrate

# ============================================================================
# Stage 2: Runtime
# ============================================================================
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy binaries
COPY --from=build /app/server /app/server
COPY --from=build /app/worker /app/worker
COPY --from=build /app/migrate /app/migrate

# Copy migrations
COPY --from=build /src/migrations /app/migrations

# Copy static assets
COPY --from=build /src/web/static /app/web/static

EXPOSE 8080

CMD ["/app/server"]
