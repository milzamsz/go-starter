package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/milzam/go-starter/internal/config"
	"github.com/milzam/go-starter/internal/database"
	"github.com/milzam/go-starter/internal/sqlc"
)

func main() {
	fmt.Println("Seeding test users...")

	// 1. Load config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// 2. Connect to database
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := database.NewPool(ctx, cfg.DB)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	queries := sqlc.New(pool)

	// Seed Admin
	seedUser(ctx, queries, "admin@test.com", "password", "System Admin", sqlc.UserRoleAdmin)

	// Seed Standard User
	seedUser(ctx, queries, "user@test.com", "password", "Test User", sqlc.UserRoleUser)

	fmt.Println("Seeding completed successfully!")
}

func seedUser(ctx context.Context, queries *sqlc.Queries, email, password, name string, role sqlc.UserRole) {
	u, err := queries.GetUserByEmail(ctx, email)
	if err == nil {
		fmt.Printf("User %s already exists. Updating role to %s...\n", email, role)
		err = queries.UpdateUserRole(ctx, sqlc.UpdateUserRoleParams{
			ID:   u.ID,
			Role: role,
		})
		if err != nil {
			log.Fatalf("failed to update role for %s: %v", email, err)
		}
		return
	}

	// Hash password
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		log.Fatalf("failed to hash password: %v", err)
	}

	hashStr := string(hashed)

	// Create user
	user, err := queries.CreateUser(ctx, sqlc.CreateUserParams{
		Email:        email,
		PasswordHash: &hashStr,
		Name:         name,
		Role:         role,
	})
	if err != nil {
		log.Fatalf("failed to create user %s: %v", email, err)
	}

	// Verify email
	err = queries.UpdateUserEmailVerified(ctx, sqlc.UpdateUserEmailVerifiedParams{
		ID:            user.ID,
		EmailVerified: true,
	})
	if err != nil {
		log.Fatalf("failed to verify email for %s: %v", email, err)
	}

	fmt.Printf("Successfully created user %s with role %s (password: %s)\n", email, role, password)
}
