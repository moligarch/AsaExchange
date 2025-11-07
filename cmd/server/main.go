package main

import (
	"AsaExchange/internal/adapters/postgres"
	"AsaExchange/internal/adapters/security"
	"AsaExchange/internal/adapters/telegram"
	_ "AsaExchange/internal/bot/customer/handlers"

	// _ "AsaExchange/internal/bot/moderator/handlers"
	"AsaExchange/internal/shared/config"
	"AsaExchange/internal/shared/logger"
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// 1. Load Configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("FATAL: Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// 2. Initialize Logger
	isDevMode := cfg.AppEnv == "development"
	baseLogger := logger.New(isDevMode)
	baseLogger.Info().Msg("Logger initialized")
	baseLogger.Info().Interface("config", cfg).Msg("Configuration loaded")

	// 3. Initialize Services & Context
	// Create a context that listens for OS shutdown signals
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	keyBytes, err := hex.DecodeString(cfg.EncryptionKey)
	if err != nil {
		baseLogger.Fatal().Err(err).Msg("Failed to decode encryption_key")
	}
	secSvc, err := security.NewAESService(keyBytes, &baseLogger)
	if err != nil {
		baseLogger.Fatal().Err(err).Msg("Failed to initialize security service")
	}

	db, err := postgres.NewDB(ctx, cfg.Postgres.URL, &baseLogger)
	if err != nil {
		baseLogger.Fatal().Err(err).Msg("Failed to initialize database")
	}
	defer db.Close()

	// 4. Initialize Repositories
	userRepo := postgres.NewUserRepository(db, secSvc, &baseLogger)
	_ = postgres.NewUserBankAccountRepository(db, secSvc, &baseLogger)

	// 5. Initialize Bot Orchestrator
	orchestrator := telegram.NewOrchestrator(
		cfg,
		userRepo,
		&baseLogger,
	)

	// 6. Start Bot Orchestrator
	baseLogger.Info().Msg("Application starting...")
	if err := orchestrator.Start(ctx); err != nil {
		baseLogger.Error().Err(err).Msg("Bot orchestrator failed")
	}

	baseLogger.Info().Msg("Application shutting down")
}
