package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/kelseyhightower/envconfig"
)

// Config holds all application configuration, loaded from environment variables.
type Config struct {
	App    AppConfig
	HTTP   HTTPConfig
	DB     DBConfig
	Redis  RedisConfig
	Auth   AuthConfig
	OAuth  OAuthConfig
	Stripe StripeConfig
	Email  EmailConfig
	Queue  QueueConfig
}

// AppConfig holds general application settings.
type AppConfig struct {
	Env                string `envconfig:"APP_ENV" default:"development"`
	Name               string `envconfig:"APP_NAME" default:"go-starter"`
	URL                string `envconfig:"APP_URL" default:"http://localhost:8080"`
	CORSAllowedOrigins string `envconfig:"CORS_ALLOWED_ORIGINS" default:"http://localhost:8080"`
}

// IsProd reports whether the application is running in production.
func (c AppConfig) IsProd() bool { return c.Env == "production" }

// IsDev reports whether the application is running in development.
func (c AppConfig) IsDev() bool { return c.Env == "development" }

// AllowedOrigins parses CORS_ALLOWED_ORIGINS into a cleaned slice.
func (c AppConfig) AllowedOrigins() []string {
	parts := strings.Split(c.CORSAllowedOrigins, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		v := strings.TrimSpace(p)
		if v != "" {
			out = append(out, v)
		}
	}
	if len(out) == 0 {
		return []string{"http://localhost:8080"}
	}
	return out
}

// HTTPConfig holds HTTP server settings.
type HTTPConfig struct {
	Host         string        `envconfig:"HTTP_HOST" default:"0.0.0.0"`
	Port         int           `envconfig:"HTTP_PORT" default:"8080"`
	ReadTimeout  time.Duration `envconfig:"HTTP_READ_TIMEOUT" default:"15s"`
	WriteTimeout time.Duration `envconfig:"HTTP_WRITE_TIMEOUT" default:"15s"`
	IdleTimeout  time.Duration `envconfig:"HTTP_IDLE_TIMEOUT" default:"60s"`
}

// Addr returns the host:port string for the HTTP server.
func (c HTTPConfig) Addr() string { return fmt.Sprintf("%s:%d", c.Host, c.Port) }

// DBConfig holds PostgreSQL connection pool settings.
type DBConfig struct {
	Host              string        `envconfig:"DB_HOST" default:"localhost"`
	Port              int           `envconfig:"DB_PORT" default:"5432"`
	User              string        `envconfig:"DB_USER" default:"postgres"`
	Password          string        `envconfig:"DB_PASSWORD" default:"postgres"`
	Name              string        `envconfig:"DB_NAME" default:"gostarter"`
	SSLMode           string        `envconfig:"DB_SSL_MODE" default:"disable"`
	MaxConns          int32         `envconfig:"DB_MAX_CONNS" default:"25"`
	MinConns          int32         `envconfig:"DB_MIN_CONNS" default:"5"`
	MaxConnLifetime   time.Duration `envconfig:"DB_MAX_CONN_LIFETIME" default:"1h"`
	MaxConnIdleTime   time.Duration `envconfig:"DB_MAX_CONN_IDLE_TIME" default:"30m"`
	HealthCheckPeriod time.Duration `envconfig:"DB_HEALTH_CHECK_PERIOD" default:"1m"`
}

// DSN returns a PostgreSQL connection string.
func (c DBConfig) DSN() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		c.User, c.Password, c.Host, c.Port, c.Name, c.SSLMode)
}

// RedisConfig holds Redis connection settings.
type RedisConfig struct {
	Addr     string `envconfig:"REDIS_ADDR" default:"localhost:6379"`
	Password string `envconfig:"REDIS_PASSWORD" default:""`
	DB       int    `envconfig:"REDIS_DB" default:"0"`
}

// AuthConfig holds authentication and session settings.
type AuthConfig struct {
	JWTSecret     string        `envconfig:"JWT_SECRET" required:"true"`
	AccessExpiry  time.Duration `envconfig:"JWT_ACCESS_EXPIRY" default:"15m"`
	RefreshExpiry time.Duration `envconfig:"JWT_REFRESH_EXPIRY" default:"168h"`
	SessionSecret string        `envconfig:"SESSION_SECRET" required:"true"`
	SessionMaxAge int           `envconfig:"SESSION_MAX_AGE" default:"86400"`
	BcryptCost    int           `envconfig:"BCRYPT_COST" default:"12"`
}

// OAuthConfig holds OAuth provider credentials.
type OAuthConfig struct {
	GoogleClientID     string `envconfig:"OAUTH_GOOGLE_CLIENT_ID"`
	GoogleClientSecret string `envconfig:"OAUTH_GOOGLE_CLIENT_SECRET"`
	GoogleRedirectURL  string `envconfig:"OAUTH_GOOGLE_REDIRECT_URL"`
	GithubClientID     string `envconfig:"OAUTH_GITHUB_CLIENT_ID"`
	GithubClientSecret string `envconfig:"OAUTH_GITHUB_CLIENT_SECRET"`
	GithubRedirectURL  string `envconfig:"OAUTH_GITHUB_REDIRECT_URL"`
}

// StripeConfig holds Stripe billing integration settings.
type StripeConfig struct {
	SecretKey     string `envconfig:"STRIPE_SECRET_KEY"`
	WebhookSecret string `envconfig:"STRIPE_WEBHOOK_SECRET"`
	PriceID       string `envconfig:"STRIPE_PRICE_ID"`
	SuccessURL    string `envconfig:"STRIPE_SUCCESS_URL"`
	CancelURL     string `envconfig:"STRIPE_CANCEL_URL"`
}

// EmailConfig holds SMTP email sending settings.
type EmailConfig struct {
	Host     string `envconfig:"SMTP_HOST" default:"localhost"`
	Port     int    `envconfig:"SMTP_PORT" default:"1025"`
	Username string `envconfig:"SMTP_USERNAME"`
	Password string `envconfig:"SMTP_PASSWORD"`
	From     string `envconfig:"SMTP_FROM" default:"noreply@example.com"`
	FromName string `envconfig:"SMTP_FROM_NAME" default:"GoStarter"`
}

// QueueConfig holds background task queue settings.
type QueueConfig struct {
	Concurrency int `envconfig:"ASYNQ_CONCURRENCY" default:"10"`
}

// Load reads configuration from environment variables.
// It returns an error if any required variable is missing.
func Load() (*Config, error) {
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}
	return &cfg, nil
}
