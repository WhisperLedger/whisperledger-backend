// Command bootstrap-admin provisions a known user as a platform administrator.
// It is intentionally explicit: the caller must supply all three environment
// variables and the command never logs credentials or database configuration.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"whisperledger-backend/internal/domain"
	"whisperledger-backend/internal/pkg/hash"
	"whisperledger-backend/internal/repository/postgres"
)

func main() {
	email := strings.TrimSpace(os.Getenv("ADMIN_EMAIL"))
	password := os.Getenv("ADMIN_PASSWORD")
	databaseURL := os.Getenv("DATABASE_URL")

	if email == "" || password == "" || databaseURL == "" {
		log.Fatal("ADMIN_EMAIL, ADMIN_PASSWORD, and DATABASE_URL must be configured")
	}
	if len(password) < 16 {
		log.Fatal("ADMIN_PASSWORD must be at least 16 characters")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pool, err := postgres.NewDB(ctx, databaseURL)
	if err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	defer pool.Close()

	passwordHash, err := hash.HashPassword(password)
	if err != nil {
		log.Fatalf("hash password: %v", err)
	}

	tag, err := pool.Exec(ctx, `
		UPDATE users
		SET password_hash = $1, role = $2, is_active = true, status = 'active', updated_at = NOW()
		WHERE LOWER(email) = LOWER($3)
	`, passwordHash, domain.RolePlatformAdmin, email)
	if err != nil {
		log.Fatalf("provision administrator: %v", err)
	}
	if tag.RowsAffected() != 1 {
		log.Fatal("administrator account was not found")
	}

	fmt.Printf("Platform administrator provisioned for %s\n", email)
}
