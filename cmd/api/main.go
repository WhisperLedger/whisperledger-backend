package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"whisperledger-backend/internal/config"
	"whisperledger-backend/internal/handler"
	v1 "whisperledger-backend/internal/handler/v1"
	"whisperledger-backend/internal/middleware"
	"whisperledger-backend/internal/pkg/token"
	"whisperledger-backend/internal/repository/postgres"
	"whisperledger-backend/internal/service"
)

func main() {
	cfg := config.Load()

	// Logger
	var zapLog *zap.Logger
	var err error
	if cfg.Environment == "production" {
		zapLog, err = zap.NewProduction()
	} else {
		zapLog, err = zap.NewDevelopment()
	}
	if err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer zapLog.Sync()

	zapLog.Info("Starting WhisperLedger Backend API Service...",
		zap.String("environment", cfg.Environment),
		zap.String("port", cfg.Port),
	)

	// Database Connection Pool
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	dbPool, err := postgres.NewDB(ctx, cfg.DatabaseURL)
	if err != nil {
		zapLog.Warn("Database connection failed, will attempt again on requests or docker spin-up", zap.Error(err))
	} else {
		zapLog.Info("Successfully connected to PostgreSQL database cluster")

		// Apply database schema migrations if available
		if migrationSQL, err := os.ReadFile("migrations/000001_init.up.sql"); err == nil {
			if _, mErr := dbPool.Exec(context.Background(), string(migrationSQL)); mErr != nil {
				zapLog.Warn("Auto-migration schema notices", zap.Error(mErr))
			} else {
				zapLog.Info("PostgreSQL relational schema verified up-to-date")
			}
		}
	}

	// Security & Token Maker
	tokenMaker := token.NewTokenMaker(cfg.JWTSecret, cfg.AccessTokenExpiry, cfg.RefreshTokenExpiry)

	// Repositories
	userRepo := postgres.NewUserRepository(dbPool)
	expenseRepo := postgres.NewExpenseRepository(dbPool)
	householdRepo := postgres.NewHouseholdRepository(dbPool)
	receivableRepo := postgres.NewReceivableRepository(dbPool)

	// Services
	authService := service.NewAuthService(userRepo, tokenMaker)
	ledgerService := service.NewLedgerService(expenseRepo, householdRepo)
	householdService := service.NewHouseholdService(householdRepo, userRepo)
	recoveryService := service.NewRecoveryService(receivableRepo)
	detectiveService := service.NewDetectiveService(expenseRepo, receivableRepo, householdRepo)
	adminService := service.NewAdminService(dbPool, userRepo)

	// Middleware
	authMiddleware := middleware.NewAuthMiddleware(tokenMaker)

	// Handlers
	authHandler := v1.NewAuthHandler(authService)
	expenseHandler := v1.NewExpenseHandler(ledgerService)
	householdHandler := v1.NewHouseholdHandler(householdService, authService)
	recoveryHandler := v1.NewRecoveryHandler(recoveryService)
	detectiveHandler := v1.NewDetectiveHandler(detectiveService, authService)
	adminHandler := v1.NewAdminHandler(adminService)

	// Router
	r := handler.NewRouter(handler.RouterConfig{
		Config:           cfg,
		Logger:           zapLog,
		AuthMiddleware:   authMiddleware,
		AuthHandler:      authHandler,
		ExpenseHandler:   expenseHandler,
		HouseholdHandler: householdHandler,
		RecoveryHandler:  recoveryHandler,
		DetectiveHandler: detectiveHandler,
		AdminHandler:     adminHandler,
	})

	// HTTP Server
	server := &http.Server{
		Addr:         "0.0.0.0:" + cfg.Port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	serverErrors := make(chan error, 1)
	go func() {
		zapLog.Info("WhisperLedger API ready to accept traffic", zap.String("addr", server.Addr))
		serverErrors <- server.ListenAndServe()
	}()

	// Graceful Shutdown
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			zapLog.Fatal("Server encountered fatal error", zap.Error(err))
		}

	case sig := <-shutdown:
		zapLog.Info("Shutdown signal received, draining active connections...", zap.String("signal", sig.String()))

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			zapLog.Error("Graceful shutdown failed, forcing exit", zap.Error(err))
			_ = server.Close()
		}

		if dbPool != nil {
			dbPool.Close()
		}
		zapLog.Info("WhisperLedger API server terminated cleanly")
	}
}
