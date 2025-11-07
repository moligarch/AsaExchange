package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// --- Structs ---

type PollingConfig struct {
	WorkerPoolSize int `mapstructure:"worker_pool_size"`
}

type WebhookConfig struct {
	ListenPort int    `mapstructure:"listen_port"`
	URL        string `mapstructure:"url"`
}

type BotConnectionConfig struct {
	Mode    string        `mapstructure:"mode"`
	Polling PollingConfig `mapstructure:"polling"`
	Webhook WebhookConfig `mapstructure:"webhook"`
}

type CountryConfig struct {
	Title    string `mapstructure:"title"`
	Strategy string `mapstructure:"strategy"`
}

type CustomerBotConfig struct {
	Token             string                   `mapstructure:"token"`
	Connection        BotConnectionConfig      `mapstructure:"connection"`
	CountryStrategies map[string]CountryConfig `mapstructure:"country_strategies"`
}

type ModeratorBotConfig struct {
	Token      string              `mapstructure:"token"`
	ChannelID  int64               `mapstructure:"channel_id"`
	Connection BotConnectionConfig `mapstructure:"connection"`
}

type BotConfig struct {
	Customer  CustomerBotConfig  `mapstructure:"customer"`
	Moderator ModeratorBotConfig `mapstructure:"moderator"`
}

type PostgresConfig struct {
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DB       string `mapstructure:"db"`
	URL      string `mapstructure:"url"`
}

type Config struct {
	AppEnv        string         `mapstructure:"app_env"`
	EncryptionKey string         `mapstructure:"encryption_key"`
	Postgres      PostgresConfig `mapstructure:"postgres"`
	Bot           BotConfig      `mapstructure:"bot"`
}

// findProjectRoot
func findProjectRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for i := 0; i < 5; i++ { // Search up 5 levels max
		if _, err := os.Stat(filepath.Join(cwd, "go.mod")); err == nil {
			return cwd, nil // Found it
		}
		// Go up one directory
		parent := filepath.Dir(cwd)
		if parent == cwd {
			// Reached root without finding
			break
		}
		cwd = parent
	}

	// Fallback to CWD if go.mod not found
	cwd, err = os.Getwd()
	if err != nil {
		return "", err
	}
	return cwd, nil
}

// Load loads configuration from config.yaml ONLY
func Load() (*Config, error) {
	// 1. Find project root
	projectRoot, err := findProjectRoot()
	if err != nil {
		return nil, fmt.Errorf("error finding project root: %w", err)
	}

	// 2. Set up Viper
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(projectRoot) // Look for config.yaml in the root

	// 3. Read the config.yaml file.
	// This is now a fatal error if it's not found.
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("FATAL: failed to read config.yaml: %w", err)
	}

	// 4. Set defaults (for any keys missing from the yaml)
	v.SetDefault("app_env", "development")
	v.SetDefault("bot.customer.connection.mode", "polling")
	v.SetDefault("bot.customer.connection.polling.worker_pool_size", 5)
	v.SetDefault("bot.moderator.connection.mode", "polling")
	v.SetDefault("bot.moderator.connection.polling.worker_pool_size", 1)

	// 5. Unmarshal the config
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// 6. Validation (Updated to check new paths)
	if cfg.EncryptionKey == "" {
		return nil, errors.New("encryption_key is not set in config.yaml")
	}
	if len(cfg.EncryptionKey) != 64 {
		return nil, errors.New("encryption_key must be a 64-character hex string")
	}
	if cfg.Postgres.URL == "" {
		return nil, errors.New("postgres.url is not set in config.yaml")
	}
	if cfg.Bot.Customer.Token == "" {
		return nil, errors.New("bot.customer.token is not set in config.yaml")
	}
	if cfg.Bot.Moderator.Token == "" {
		return nil, errors.New("bot.moderator.token is not set in config.yaml")
	}
	if cfg.Bot.Customer.Connection.Mode != "polling" && cfg.Bot.Customer.Connection.Mode != "webhook" {
		return nil, errors.New("bot.mode must be 'polling' or 'webhook' in config.yaml")
	}
	if cfg.Bot.Moderator.Connection.Mode != "polling" && cfg.Bot.Moderator.Connection.Mode != "webhook" {
		return nil, errors.New("bot.mode must be 'polling' or 'webhook' in config.yaml")
	}
	if len(cfg.Bot.Customer.CountryStrategies) == 0 {
		return nil, errors.New("bot.country_strategies is not defined in config.yaml")
	}

	return &cfg, nil
}
