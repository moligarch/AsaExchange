package main

import (
	"AsaExchange/internal/adapters/postgres"
	"AsaExchange/internal/adapters/security"
	"AsaExchange/internal/shared/config"
	"AsaExchange/internal/shared/logger"
	"context"
	"encoding/hex"
	"fmt"
	"os"
)

func main() {
	// 1. Load Configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("FATAL: Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// 2. Initialize Logger
	isDevMode := cfg.AppEnv == "dev"
	baseLogger := logger.New(isDevMode)
	baseLogger.Info().Msg("Logger initialized")

	// 3. Print loaded config
	baseLogger.Info().
		Str("app_env", cfg.AppEnv).
		Int("key_length", len(cfg.EncryptionKey)).
		Msg("Configuration loaded")

	// 4. Initialize the Security Service (NEW PATTERN)
	keyBytes, err := hex.DecodeString(cfg.EncryptionKey)
	if err != nil {
		baseLogger.Fatal().Err(err).Msg("Failed to decode ENCRYPTION_KEY. It must be hex-encoded.")
	}

	// Pass the baseLogger. The service will add its own context.
	secSvc, err := security.NewAESService(keyBytes, &baseLogger)
	if err != nil {
		baseLogger.Fatal().Err(err).Msg("Failed to initialize security service")
	}

	// 5. Initialize Database
	ctx := context.Background()
	db, err := postgres.NewDB(ctx, cfg.DatabaseURL, &baseLogger)
	if err != nil {
		baseLogger.Fatal().Err(err).Msg("Failed to initialize database")
	}
	defer db.Close()

	// 6. Initialize Repositories
	userRepo := postgres.NewUserRepository(db, secSvc, &baseLogger)

	// --- NEW: Initialize UserBankAccountRepository ---
	bankRepo := postgres.NewUserBankAccountRepository(db, secSvc, &baseLogger)

	baseLogger.Info().Msg("All services initialized successfully")

	// --- Example Test (can be removed later) ---
	baseLogger.Info().Msg("Testing user repository...")
	testUser, err := userRepo.GetByTelegramID(ctx, 12345)
	if err != nil {
		baseLogger.Error().Err(err).Msg("Failed to query test user")
	}
	if testUser == nil {
		baseLogger.Info().Msg("Test user '12345' not found (this is normal on first run)")
	} else {
		baseLogger.Info().Str("user_id", testUser.ID.String()).Msg("Found test user")

		// --- NEW: Test bank repo ---
		accts, err := bankRepo.GetByUserID(ctx, testUser.ID)
		if err != nil {
			baseLogger.Error().Err(err).Str("user_id", testUser.ID.String()).Msg("Failed to get bank accounts for test user")
		} else {
			baseLogger.Info().Int("count", len(accts)).Str("user_id", testUser.ID.String()).Msg("Found bank accounts for test user")
		}
	}

	baseLogger.Info().Msg("Application started. (No server running yet)")
}
