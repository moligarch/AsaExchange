package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

// Config holds all configuration for the application.
type Config struct {
	AppEnv        string
	EncryptionKey string
}

// Load loads configuration from environment variables.
func Load() (*Config, error) {

	// 1. Load .env file into the process environment
	// We check for the error to be sure the file was found.
	if err := godotenv.Load(); err != nil {
		// If the file just doesn't exist, that's fine in prod.
		// But if it's any other error, we should know.
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("error loading .env file: %w", err)
		}
		// If .env is not found, we just proceed,
		// relying on OS-set env vars.
	}

	// 2. Explicitly bind viper keys to env var names
	// This tells viper: "The key 'app.env' is fed by
	// the environment variable 'APP_ENV'"
	err := viper.BindEnv("app.env", "APP_ENV")
	if err != nil {
		return nil, fmt.Errorf("could not bind app.env: %w", err)
	}
	err = viper.BindEnv("encryption.key", "ENCRYPTION_KEY")
	if err != nil {
		return nil, fmt.Errorf("could not bind encryption.key: %w", err)
	}

	// 3. Set defaults
	viper.SetDefault("app.env", "dev")

	// 4. Get values directly from viper
	cfg := Config{
		AppEnv:        viper.GetString("app.env"),
		EncryptionKey: viper.GetString("encryption.key"),
	}

	// 5. Validation
	if cfg.EncryptionKey == "" {
		// This is the error you are seeing.
		// For debugging, let's check the env var directly
		directEnv, exists := os.LookupEnv("ENCRYPTION_KEY")
		if !exists {
			return nil, errors.New("ENCRYPTION_KEY is not set in environment or .env file")
		}
		if directEnv == "" {
			return nil, errors.New("ENCRYPTION_KEY is set but is an empty string")
		}
		// If we get here, godotenv worked but viper failed.
		return nil, fmt.Errorf("ENCRYPTION_KEY was set, but viper.GetString failed. Val=%s", directEnv)
	}

	if len(cfg.EncryptionKey) != 64 {
		return nil, fmt.Errorf("ENCRYPTION_KEY must be a 64-character hex string (32 bytes), but got %d chars", len(cfg.EncryptionKey))
	}

	return &cfg, nil
}
