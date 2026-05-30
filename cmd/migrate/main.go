package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/milzam/go-starter/internal/config"
	"github.com/milzam/go-starter/migrations"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("migrate: %v", err)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	db, err := sql.Open("pgx", cfg.DB.DSN())
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()

	if err := db.PingContext(context.Background()); err != nil {
		return fmt.Errorf("pinging database: %w", err)
	}

	goose.SetBaseFS(migrations.FS)

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("setting dialect: %w", err)
	}

	flag.Parse()
	args := flag.Args()

	command := "up"
	if len(args) > 0 {
		command = args[0]
	}

	var cmdArgs []string
	if len(args) > 1 {
		cmdArgs = args[1:]
	}

	dir := "."

	switch command {
	case "up":
		return goose.Up(db, dir)
	case "up-one":
		return goose.UpByOne(db, dir)
	case "down":
		return goose.Down(db, dir)
	case "reset":
		return goose.Reset(db, dir)
	case "status":
		return goose.Status(db, dir)
	case "version":
		return goose.Version(db, dir)
	case "create":
		if len(cmdArgs) < 1 {
			return fmt.Errorf("create requires a migration name")
		}
		return goose.Create(db, "migrations", cmdArgs[0], "sql")
	default:
		return fmt.Errorf("unknown command: %s (valid: up, up-one, down, reset, status, version, create)", command)
	}

	// Unreachable but keeps the compiler happy
	return nil
}

func init() {
	// Suppress usage output
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: migrate [command] [args]\n\n")
		fmt.Fprintf(os.Stderr, "Commands:\n")
		fmt.Fprintf(os.Stderr, "  up        Migrate to the latest version\n")
		fmt.Fprintf(os.Stderr, "  up-one    Migrate one version up\n")
		fmt.Fprintf(os.Stderr, "  down      Roll back one migration\n")
		fmt.Fprintf(os.Stderr, "  reset     Roll back all migrations\n")
		fmt.Fprintf(os.Stderr, "  status    Show migration status\n")
		fmt.Fprintf(os.Stderr, "  version   Show current migration version\n")
		fmt.Fprintf(os.Stderr, "  create    Create a new migration (requires name)\n")
	}
}
