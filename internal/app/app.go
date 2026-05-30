package app

import (
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/milzam/go-starter/internal/auth"
	"github.com/milzam/go-starter/internal/authz"
	"github.com/milzam/go-starter/internal/config"
	"github.com/milzam/go-starter/internal/modules/billing"
	"github.com/milzam/go-starter/internal/modules/users"
	"github.com/milzam/go-starter/internal/queue"
	"github.com/milzam/go-starter/internal/sqlc"
)

// App is the central application container that holds all shared dependencies.
// It is constructed via New() with explicit dependency injection — no globals.
type App struct {
	Config         *config.Config
	DB             *pgxpool.Pool
	Logger         *slog.Logger
	Queries        *sqlc.Queries
	AuthService    *auth.Service
	UsersService   *users.Service
	BillingService *billing.Service
	Enforcer       *authz.Enforcer
	Queue          *queue.Client
}

// New creates a new App with the provided dependencies.
func New(
	cfg *config.Config,
	db *pgxpool.Pool,
	logger *slog.Logger,
	queries *sqlc.Queries,
	authService *auth.Service,
	usersService *users.Service,
	billingService *billing.Service,
	enforcer *authz.Enforcer,
	queueClient *queue.Client,
) *App {
	return &App{
		Config:         cfg,
		DB:             db,
		Logger:         logger,
		Queries:        queries,
		AuthService:    authService,
		UsersService:   usersService,
		BillingService: billingService,
		Enforcer:       enforcer,
		Queue:          queueClient,
	}
}

// Close releases all resources held by the App.
// It should be called during graceful shutdown (typically deferred in main).
func (a *App) Close() {
	if a.Queue != nil {
		_ = a.Queue.Close()
	}
	if a.DB != nil {
		a.DB.Close()
	}
}
