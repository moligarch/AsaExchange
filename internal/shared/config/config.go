package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

// --- New/Updated Structs ---

// Struct for pooling settings from config.yaml
type PollingConfig struct {
	WorkerPoolSize int `mapstructure:"worker_pool_size"`
}

// Struct for webhook settings from config.yaml
type WebhookConfig struct {
	ListenPort int    `mapstructure:"listen_port"` // From yaml
	URL        string `mapstructure:"WEBHOOK_URL"` // From .env
}

type BotConfig struct {
	Token     string        `mapstructure:"BOT_TOKEN"`     // From .env
	ModToken  string        `mapstructure:"MOD_BOT_TOKEN"` // From .env
	ChannelID int64         `mapstructure:"CHANNEL_ID"`    // From .env
	Mode      string        `mapstructure:"mode"`          // From yaml
	Polling   PollingConfig `mapstructure:"polling"`       // From yaml
	Webhook   WebhookConfig `mapstructure:"webhook"`       // From yaml
}

type Config struct {
	AppEnv        string    `mapstructure:"app_env"`        // From yaml
	EncryptionKey string    `mapstructure:"ENCRYPTION_KEY"` // From .env
	DatabaseURL   string    `mapstructure:"DATABASE_URL"`   // From .env
	Bot           BotConfig `mapstructure:"bot"`
}

// Load loads configuration from config.yaml AND .env
func Load() (*Config, error) {
	// 1. Load .env file (SECRETS)
	if err := godotenv.Load(); err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("error loading .env file: %w", err)
		}
	}

	// 2. Set up Viper
	v := viper.New()

	// 3. Load config.yaml (NON-SECRETS)
	v.SetConfigName("config") // File name: config.yaml
	v.SetConfigType("yaml")
	v.AddConfigPath(".") // Look in current directory
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config.yaml: %w", err)
		}
	}

	// 4. Set Viper to read from Environment (for SECRETS)
	v.AutomaticEnv()
	// This maps env vars to the mapstructure tags
	// e.g. BOT_TOKEN maps to `mapstructure:"BOT_TOKEN"`
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// 5. Set defaults
	v.SetDefault("app_env", "development")
	v.SetDefault("bot.mode", "polling")
	v.SetDefault("bot.polling.worker_pool_size", 5)
	v.SetDefault("bot.webhook.listen_port", 8443) // Default port

	// 6. Unmarshal all config sources into our struct
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		// This is the only way to debug viper's unmarshal
		// fmt.Println("--- DEBUG VIPER SETTINGS ---")
		// v.Debug()
		// fmt.Println("------------------------------")
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// 7. Validation
	if cfg.EncryptionKey == "" {
		return nil, errors.New("ENCRYPTION_KEY is not set (from .env or OS)")
	}
	if len(cfg.EncryptionKey) != 64 {
		return nil, errors.New("ENCRYPTION_KEY must be a 64-character hex string")
	}
	if cfg.DatabaseURL == "" {
		return nil, errors.New("DATABASE_URL is not set (from .env or OS)")
	}
	if cfg.Bot.Token == "" {
		return nil, errors.New("BOT_TOKEN is not set (from .env or OS)")
	}
	if cfg.Bot.Mode != "polling" && cfg.Bot.Mode != "webhook" {
		return nil, errors.New("bot.mode must be 'polling' or 'webhook' in config.yaml")
	}
	// --- UPDATED VALIDATION ---
	if cfg.Bot.Mode == "webhook" && cfg.Bot.Webhook.URL == "" {
		return nil, errors.New("WEBHOOK_URL must be set in .env when BOT_MODE is 'webhook'")
	}
	if cfg.Bot.Polling.WorkerPoolSize <= 0 {
		return nil, errors.New("bot.polling.worker_pool_size must be > 0")
	}

	return &cfg, nil
}
