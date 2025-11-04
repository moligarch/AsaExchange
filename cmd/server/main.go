package main

import (
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

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
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
		baseLogger.Fatal().Err(err).Msg("Failed to decode ENCRYPTION_KEY")
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

	// 5. Initialize Bot API Object
	botAPI, err := tgbotapi.NewBotAPI(cfg.Bot.Token)
	if err != nil {
		baseLogger.Fatal().Err(err).Msg("Failed to create Bot API")
	}
	botAPI.Debug = cfg.AppEnv == "development"

	baseLogger.Info().
		Str("bot_username", botAPI.Self.UserName).
		Str("mode", cfg.Bot.Mode).
		Msg("Telegram Bot API connected")

	// 6. Initialize Bot Client (Adapter)
	botClient := telegram.NewClient(botAPI, &baseLogger)

	// 7. Initialize Bot Router (Facade)
	botRouter := telegram.NewRouter(userRepo, &baseLogger)

	// 8. Register Handlers (Plugins)
	baseLogger.Info().Msg("Bot router initialized (no handlers registered yet)")
	baseLogger.Info().Msg("All services initialized successfully")

	// 9. Example Test: Set Menu Commands
	err = botClient.SetMenuCommands(ctx, 0, false) // 0 = global
	if err != nil {
		baseLogger.Error().Err(err).Msg("Failed to set menu commands")
	} else {
		baseLogger.Info().Msg("Successfully set user menu commands")
	}

	// 10. Initialize Bot Server (Launcher)
	botServer := telegram.NewBotServer(
		botAPI,
		botRouter,
		&cfg.Bot,
		&baseLogger,
	)

	// 11. Start Bot Launcher
	baseLogger.Info().Msg("Application starting...")
	if err := botServer.Start(ctx); err != nil {
		// This will only log on a non-graceful shutdown (e.g., failed to set webhook)
		baseLogger.Error().Err(err).Msg("Bot server failed")
	}

	// The context was cancelled (e.g., Ctrl+C), botServer.Start() returned
	baseLogger.Info().Msg("Application shutting down")
}
