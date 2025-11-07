package telegram

import (
	"AsaExchange/internal/bot/customer"
	"AsaExchange/internal/bot/moderator"
	"AsaExchange/internal/core/ports"
	"AsaExchange/internal/shared/config"
	"context"
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/rs/zerolog"
)

// Orchestrator manages all bot servers.
type Orchestrator struct {
	cfg        *config.Config
	userRepo   ports.UserRepository
	baseLogger *zerolog.Logger
	wg         sync.WaitGroup
}

// NewOrchestrator creates a new bot orchestrator.
func NewOrchestrator(
	cfg *config.Config,
	userRepo ports.UserRepository,
	baseLogger *zerolog.Logger,
) *Orchestrator {
	return &Orchestrator{
		cfg:        cfg,
		userRepo:   userRepo,
		baseLogger: baseLogger,
	}
}

// Start launches all bot servers and waits for them to complete.
func (o *Orchestrator) Start(ctx context.Context) error {
	o.wg.Add(2) // We are launching 2 servers

	// --- Start Customer Bot ---
	go func() {
		defer o.wg.Done()
		if err := o.startCustomerBot(ctx); err != nil {
			o.baseLogger.Error().Err(err).Msg("CustomerBot failed")
		}
	}()

	// --- Start Moderator Bot ---
	go func() {
		defer o.wg.Done()
		if err := o.startModeratorBot(ctx); err != nil {
			o.baseLogger.Error().Err(err).Msg("ModeratorBot failed")
		}
	}()

	o.wg.Wait() // Wait for both goroutines to finish
	return nil
}

// startCustomerBot initializes and runs the customer-facing bot.
func (o *Orchestrator) startCustomerBot(ctx context.Context) error {
	log := o.baseLogger.With().Str("bot", "customer").Logger()
	cfg := &o.cfg.Bot.Customer

	// 1. Create API
	api, err := tgbotapi.NewBotAPI(cfg.Token)
	if err != nil {
		return err
	}
	api.Debug = o.cfg.AppEnv == "development"
	log.Info().Str("username", api.Self.UserName).Msg("Bot API connected")

	// 2. Create Client (Adapter)
	client := NewClient(api, &log)

	// 3. Create Router
	router := customer.NewCustomerRouter(o.userRepo, client, &log)

	// 4. Register Handlers
	customer.RegisterAllHandlers(o.cfg, router, o.userRepo, client, &log)

	// 5. Set Menu
	client.SetMenuCommands(ctx, 0, false)

	// 6. Create and Start Server
	server := customer.NewCustomerServer(api, router, &cfg.Connection, &log)
	return server.Start(ctx)
}

// startModeratorBot initializes and runs the internal-facing bot.
func (o *Orchestrator) startModeratorBot(ctx context.Context) error {
	log := o.baseLogger.With().Str("bot", "moderator").Logger()
	cfg := &o.cfg.Bot.Moderator

	// 1. Create API
	api, err := tgbotapi.NewBotAPI(cfg.Token)
	if err != nil {
		return err
	}
	api.Debug = o.cfg.AppEnv == "development"
	log.Info().Str("username", api.Self.UserName).Msg("Bot API connected")

	// 2. Create Client (Adapter)
	client := NewClient(api, &log)

	// 3. Create Router
	router := moderator.NewModeratorRouter(o.userRepo, client, &log)

	// 4. Register Handlers
	moderator.RegisterAllHandlers(o.cfg, router, o.userRepo, client, &log)

	// 5. Set Menu
	client.SetMenuCommands(ctx, 0, true) // true = admin menu

	// 6. Create and Start Server
	server := moderator.NewModeratorServer(api, router, &cfg.Connection, &log)
	return server.Start(ctx)
}
