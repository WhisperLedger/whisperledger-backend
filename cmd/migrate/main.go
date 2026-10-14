package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"whisperledger-backend/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Invalid migration configuration: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer conn.Close(ctx)

	// Ensure schema_migrations table exists
	_, err = conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(255) PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
	`)
	if err != nil {
		log.Fatalf("Failed to initialize schema_migrations table: %v", err)
	}

	direction := "up"
	if len(os.Args) > 1 {
		direction = os.Args[1]
	}

	if direction != "up" {
		log.Printf("Non-up migrations not executed automatically: %s", direction)
		return
	}

	// Find all *.up.sql files in migrations directory
	files, err := filepath.Glob("migrations/*.up.sql")
	if err != nil {
		log.Fatalf("Failed to scan migrations directory: %v", err)
	}
	sort.Strings(files)

	for _, file := range files {
		baseName := filepath.Base(file)
		version := strings.TrimSuffix(baseName, ".up.sql")

		var exists bool
		err := conn.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)", version).Scan(&exists)
		if err != nil {
			log.Fatalf("Failed to check migration version %s: %v", version, err)
		}

		if exists {
			log.Printf("Migration %s already applied, skipping", baseName)
			continue
		}

		sqlContent, err := os.ReadFile(file)
		if err != nil {
			log.Fatalf("Failed to read migration file %s: %v", file, err)
		}

		log.Printf("Executing migration: %s", file)
		tx, err := conn.Begin(ctx)
		if err != nil {
			log.Fatalf("Failed to start transaction for %s: %v", file, err)
		}

		if _, err := tx.Exec(ctx, string(sqlContent)); err != nil {
			_ = tx.Rollback(ctx)
			log.Fatalf("Migration failed for %s: %v", file, err)
		}

		if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", version); err != nil {
			_ = tx.Rollback(ctx)
			log.Fatalf("Failed to record migration %s: %v", file, err)
		}

		if err := tx.Commit(ctx); err != nil {
			log.Fatalf("Failed to commit migration %s: %v", file, err)
		}

		fmt.Printf("Migration %s applied successfully.\n", baseName)
	}

	log.Println("All pending database migrations completed successfully.")
}
