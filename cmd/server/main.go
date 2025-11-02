package main

import (
	"AsaExchange/internal/adapters/security"
	"AsaExchange/internal/shared/config"
	"AsaExchange/internal/shared/logger"
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

	// --- Example Test of our service ---
	baseLogger.Info().Msg("Testing encryption service from main...")
	plaintext := "this is a secret"

	ciphertext, err := secSvc.Encrypt([]byte(plaintext))
	if err != nil {
		baseLogger.Error().Err(err).Msg("Failed to encrypt")
		return
	}

	decrypted, err := secSvc.Decrypt(ciphertext)
	if err != nil {
		baseLogger.Error().Err(err).Msg("Failed to decrypt")
		return
	}

	baseLogger.Info().
		Str("original", plaintext).
		Str("decrypted", string(decrypted)).
		Msg("Encryption roundtrip successful!")
}
