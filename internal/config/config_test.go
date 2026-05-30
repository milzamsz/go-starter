package config

import (
	"testing"
	"time"
)

// setRequiredEnv sets the minimum required environment variables so Load() succeeds.
func setRequiredEnv(t *testing.T) {
	t.Helper()
	t.Setenv("JWT_SECRET", "test-jwt-secret-32-bytes-long!!")
	t.Setenv("SESSION_SECRET", "test-session-secret-32-bytes!!!")
}

func TestLoad_Defaults(t *testing.T) {
	setRequiredEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}

	// AppConfig defaults
	if cfg.App.Env != "development" {
		t.Errorf("App.Env = %q, want %q", cfg.App.Env, "development")
	}
	if cfg.App.Name != "go-starter" {
		t.Errorf("App.Name = %q, want %q", cfg.App.Name, "go-starter")
	}
	if cfg.App.URL != "http://localhost:8080" {
		t.Errorf("App.URL = %q, want %q", cfg.App.URL, "http://localhost:8080")
	}

	// HTTPConfig defaults
	if cfg.HTTP.Host != "0.0.0.0" {
		t.Errorf("HTTP.Host = %q, want %q", cfg.HTTP.Host, "0.0.0.0")
	}
	if cfg.HTTP.Port != 8080 {
		t.Errorf("HTTP.Port = %d, want %d", cfg.HTTP.Port, 8080)
	}
	if cfg.HTTP.ReadTimeout != 15*time.Second {
		t.Errorf("HTTP.ReadTimeout = %v, want %v", cfg.HTTP.ReadTimeout, 15*time.Second)
	}
	if cfg.HTTP.WriteTimeout != 15*time.Second {
		t.Errorf("HTTP.WriteTimeout = %v, want %v", cfg.HTTP.WriteTimeout, 15*time.Second)
	}
	if cfg.HTTP.IdleTimeout != 60*time.Second {
		t.Errorf("HTTP.IdleTimeout = %v, want %v", cfg.HTTP.IdleTimeout, 60*time.Second)
	}

	// DBConfig defaults
	if cfg.DB.Host != "localhost" {
		t.Errorf("DB.Host = %q, want %q", cfg.DB.Host, "localhost")
	}
	if cfg.DB.Port != 5432 {
		t.Errorf("DB.Port = %d, want %d", cfg.DB.Port, 5432)
	}
	if cfg.DB.MaxConns != 25 {
		t.Errorf("DB.MaxConns = %d, want %d", cfg.DB.MaxConns, 25)
	}
	if cfg.DB.MinConns != 5 {
		t.Errorf("DB.MinConns = %d, want %d", cfg.DB.MinConns, 5)
	}
	if cfg.DB.SSLMode != "disable" {
		t.Errorf("DB.SSLMode = %q, want %q", cfg.DB.SSLMode, "disable")
	}

	// RedisConfig defaults
	if cfg.Redis.Addr != "localhost:6379" {
		t.Errorf("Redis.Addr = %q, want %q", cfg.Redis.Addr, "localhost:6379")
	}
	if cfg.Redis.DB != 0 {
		t.Errorf("Redis.DB = %d, want %d", cfg.Redis.DB, 0)
	}

	// AuthConfig defaults (required fields are set via setRequiredEnv)
	if cfg.Auth.AccessExpiry != 15*time.Minute {
		t.Errorf("Auth.AccessExpiry = %v, want %v", cfg.Auth.AccessExpiry, 15*time.Minute)
	}
	if cfg.Auth.RefreshExpiry != 168*time.Hour {
		t.Errorf("Auth.RefreshExpiry = %v, want %v", cfg.Auth.RefreshExpiry, 168*time.Hour)
	}
	if cfg.Auth.SessionMaxAge != 86400 {
		t.Errorf("Auth.SessionMaxAge = %d, want %d", cfg.Auth.SessionMaxAge, 86400)
	}
	if cfg.Auth.BcryptCost != 12 {
		t.Errorf("Auth.BcryptCost = %d, want %d", cfg.Auth.BcryptCost, 12)
	}

	// EmailConfig defaults
	if cfg.Email.Host != "localhost" {
		t.Errorf("Email.Host = %q, want %q", cfg.Email.Host, "localhost")
	}
	if cfg.Email.Port != 1025 {
		t.Errorf("Email.Port = %d, want %d", cfg.Email.Port, 1025)
	}
	if cfg.Email.From != "noreply@example.com" {
		t.Errorf("Email.From = %q, want %q", cfg.Email.From, "noreply@example.com")
	}

	// QueueConfig defaults
	if cfg.Queue.Concurrency != 10 {
		t.Errorf("Queue.Concurrency = %d, want %d", cfg.Queue.Concurrency, 10)
	}
}

func TestLoad_MissingRequired(t *testing.T) {
	// Do not set JWT_SECRET or SESSION_SECRET — Load should fail.
	_, err := Load()
	if err == nil {
		t.Fatal("Load() should return error when required env vars are missing")
	}
}

func TestLoad_CustomValues(t *testing.T) {
	setRequiredEnv(t)

	t.Setenv("APP_ENV", "production")
	t.Setenv("APP_NAME", "my-saas")
	t.Setenv("HTTP_PORT", "9090")
	t.Setenv("HTTP_READ_TIMEOUT", "30s")
	t.Setenv("DB_HOST", "db.example.com")
	t.Setenv("DB_PORT", "5433")
	t.Setenv("DB_USER", "myuser")
	t.Setenv("DB_PASSWORD", "mypass")
	t.Setenv("DB_NAME", "mydb")
	t.Setenv("DB_SSL_MODE", "require")
	t.Setenv("DB_MAX_CONNS", "50")
	t.Setenv("REDIS_ADDR", "redis.example.com:6380")
	t.Setenv("REDIS_DB", "2")
	t.Setenv("STRIPE_SECRET_KEY", "sk_test_123")
	t.Setenv("ASYNQ_CONCURRENCY", "20")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}

	if cfg.App.Env != "production" {
		t.Errorf("App.Env = %q, want %q", cfg.App.Env, "production")
	}
	if cfg.App.Name != "my-saas" {
		t.Errorf("App.Name = %q, want %q", cfg.App.Name, "my-saas")
	}
	if cfg.HTTP.Port != 9090 {
		t.Errorf("HTTP.Port = %d, want %d", cfg.HTTP.Port, 9090)
	}
	if cfg.HTTP.ReadTimeout != 30*time.Second {
		t.Errorf("HTTP.ReadTimeout = %v, want %v", cfg.HTTP.ReadTimeout, 30*time.Second)
	}
	if cfg.DB.Host != "db.example.com" {
		t.Errorf("DB.Host = %q, want %q", cfg.DB.Host, "db.example.com")
	}
	if cfg.DB.Port != 5433 {
		t.Errorf("DB.Port = %d, want %d", cfg.DB.Port, 5433)
	}
	if cfg.DB.MaxConns != 50 {
		t.Errorf("DB.MaxConns = %d, want %d", cfg.DB.MaxConns, 50)
	}
	if cfg.Redis.Addr != "redis.example.com:6380" {
		t.Errorf("Redis.Addr = %q, want %q", cfg.Redis.Addr, "redis.example.com:6380")
	}
	if cfg.Redis.DB != 2 {
		t.Errorf("Redis.DB = %d, want %d", cfg.Redis.DB, 2)
	}
	if cfg.Stripe.SecretKey != "sk_test_123" {
		t.Errorf("Stripe.SecretKey = %q, want %q", cfg.Stripe.SecretKey, "sk_test_123")
	}
	if cfg.Queue.Concurrency != 20 {
		t.Errorf("Queue.Concurrency = %d, want %d", cfg.Queue.Concurrency, 20)
	}
}

func TestAppConfig_IsProd(t *testing.T) {
	tests := []struct {
		env  string
		want bool
	}{
		{"production", true},
		{"development", false},
		{"staging", false},
		{"", false},
	}
	for _, tt := range tests {
		c := AppConfig{Env: tt.env}
		if got := c.IsProd(); got != tt.want {
			t.Errorf("AppConfig{Env: %q}.IsProd() = %v, want %v", tt.env, got, tt.want)
		}
	}
}

func TestAppConfig_IsDev(t *testing.T) {
	tests := []struct {
		env  string
		want bool
	}{
		{"development", true},
		{"production", false},
		{"staging", false},
		{"", false},
	}
	for _, tt := range tests {
		c := AppConfig{Env: tt.env}
		if got := c.IsDev(); got != tt.want {
			t.Errorf("AppConfig{Env: %q}.IsDev() = %v, want %v", tt.env, got, tt.want)
		}
	}
}

func TestHTTPConfig_Addr(t *testing.T) {
	c := HTTPConfig{Host: "127.0.0.1", Port: 3000}
	want := "127.0.0.1:3000"
	if got := c.Addr(); got != want {
		t.Errorf("HTTPConfig.Addr() = %q, want %q", got, want)
	}
}

func TestDBConfig_DSN(t *testing.T) {
	c := DBConfig{
		Host:     "db.example.com",
		Port:     5433,
		User:     "admin",
		Password: "s3cret",
		Name:     "mydb",
		SSLMode:  "require",
	}
	want := "postgres://admin:s3cret@db.example.com:5433/mydb?sslmode=require"
	if got := c.DSN(); got != want {
		t.Errorf("DBConfig.DSN() = %q, want %q", got, want)
	}
}
