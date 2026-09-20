package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"whisperledger-backend/internal/config"
)

func main() {
	cfg := config.Load()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer conn.Close(ctx)

	direction := "up"
	if len(os.Args) > 1 {
		direction = os.Args[1]
	}

	var sqlFile string
	if direction == "down" {
		sqlFile = "migrations/000001_init.down.sql"
	} else {
		sqlFile = "migrations/000001_init.up.sql"
	}

	sqlContent, err := os.ReadFile(sqlFile)
	if err != nil {
		log.Fatalf("Failed to read migration file %s: %v", sqlFile, err)
	}

	log.Printf("Executing migration: %s", sqlFile)
	_, err = conn.Exec(ctx, string(sqlContent))
	if err != nil {
		log.Fatalf("Migration failed: %v", err)
	}

	fmt.Printf("Migration %s applied successfully.\n", sqlFile)
}
