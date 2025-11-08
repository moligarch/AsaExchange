package main

import (
	"AsaExchange/internal/adapters/eventbus"
	"AsaExchange/internal/adapters/postgres"
	"AsaExchange/internal/adapters/security"
	"AsaExchange/internal/adapters/telegram"
	"AsaExchange/internal/shared/config"
	"AsaExchange/internal/shared/logger"
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	// --- BLANK IMPORTS TO TRIGGER HANDLER REGISTRATION ---
	_ "AsaExchange/internal/bot/customer/handlers"
	_ "AsaExchange/internal/bot/moderator/handlers"
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

	// Create the EventBus first
	bus := eventbus.NewInMemoryEventBus(&baseLogger)

	// 6. Initialize Bot Orchestrator
	// Pass the bus to the constructor
	orchestrator := telegram.NewOrchestrator(
		cfg,
		userRepo,
		bus,
		&baseLogger,
	)

	// 7. Start Bot Orchestrator
	baseLogger.Info().Msg("Application starting...")
	if err := orchestrator.Start(ctx); err != nil {
		baseLogger.Error().Err(err).Msg("Bot orchestrator failed")
	}

	baseLogger.Info().Msg("Application shutting down")
}
