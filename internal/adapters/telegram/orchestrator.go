package telegram

import (
	"AsaExchange/internal/bot/customer"
	custHandle "AsaExchange/internal/bot/customer/handlers"
	"AsaExchange/internal/bot/moderator"
	modHandle "AsaExchange/internal/bot/moderator/handlers"
	"AsaExchange/internal/core/ports"
	"AsaExchange/internal/shared/config"
	"context"
	"fmt"
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/rs/zerolog"
)

// Orchestrator manages all bot servers.
type Orchestrator struct {
	cfg        *config.Config
	userRepo   ports.UserRepository
	bus        ports.EventBus
	baseLogger *zerolog.Logger
	wg         sync.WaitGroup
}

// NewOrchestrator creates a new bot orchestrator.
func NewOrchestrator(
	cfg *config.Config,
	userRepo ports.UserRepository,
	bus ports.EventBus,
	baseLogger *zerolog.Logger,
) *Orchestrator {
	return &Orchestrator{
		cfg:        cfg,
		userRepo:   userRepo,
		bus:        bus,
		baseLogger: baseLogger,
	}
}

// Start launches all bot servers and waits for them to complete.
func (o *Orchestrator) Start(ctx context.Context) error {
	// We are launching 2 main servers
	o.wg.Add(2)

	// --- 1. Create Customer Bot Dependencies ---
	custLog := o.baseLogger.With().Str("bot", "customer").Logger()
	custCfg := &o.cfg.Bot.Customer
	custAPI, err := tgbotapi.NewBotAPI(custCfg.Token)
	if err != nil {
		return fmt.Errorf("customer bot API failed: %w", err)
	}
	custAPI.Debug = o.cfg.AppEnv == "development"
	custLog.Info().Str("username", custAPI.Self.UserName).Msg("Bot API connected")
	custClient := NewClient(custAPI, &custLog)

	// --- 2. Create Moderator Bot Dependencies ---
	modLog := o.baseLogger.With().Str("bot", "moderator").Logger()
	modCfg := &o.cfg.Bot.Moderator

	// Create the ONE AND ONLY API for the moderator
	modAPI, err := tgbotapi.NewBotAPI(modCfg.Token)
	if err != nil {
		return fmt.Errorf("moderator bot API failed: %w", err)
	}
	modAPI.Debug = o.cfg.AppEnv == "development"
	modLog.Info().Str("username", modAPI.Self.UserName).Msg("Bot API (commands) connected")
	modClient := NewClient(modAPI, &modLog)

	// --- 3. Create the Shared Queue ---
	// It's injected with the bus so it can *subscribe*
	queue := NewTelegramQueue(
		custClient, // Customer client (to Publish)
		o.cfg.Bot.PrivateUploadChannelID,
		o.bus, // The event bus (to Subscribe)
		o.baseLogger,
	)

	// 4. --- Create and Subscribe Handlers ---

	// Create the Customer Router
	custRouter := customer.NewCustomerRouter(o.userRepo, custClient, &custLog)
	// Register all customer handlers (which also injects the queue)
	customer.RegisterAllHandlers(o.cfg, custRouter, o.userRepo, custClient, queue, &custLog)

	// Create the Moderator Router (which subscribes to the bus)
	modRouter := moderator.NewModeratorRouter(o.userRepo, modClient, o.bus, &modLog)
	// Register all moderator handlers (commands/callbacks)
	moderator.RegisterAllHandlers(o.cfg, modRouter, o.userRepo, modClient, o.bus, &modLog)

	// Create the Notification Handler (it's not a router plugin)
	// It uses the CUSTOMER client to send messages
	notificationHandler := custHandle.NewNotificationHandler(custClient, o.userRepo, &custLog)
	// Subscribe it to the events published by the approval_handler
	o.bus.Subscribe("user:approved", notificationHandler.HandleUserApproved)
	o.bus.Subscribe("user:rejected", notificationHandler.HandleUserRejected)

	// Create the Forwarding Handler (the queue's subscriber)
	fwdHandler := modHandle.NewForwardingHandler(
		o.cfg,
		o.userRepo,
		modClient, // Use modClient to post to the admin channel
		&modLog,
	)
	// Manually subscribe the queue to its handler
	queue.Subscribe(ctx, fwdHandler.HandleEvent)

	// --- 5. Start Customer Bot Server ---
	go func() {
		defer o.wg.Done()
		custClient.SetMenuCommands(ctx, 0, false)

		server := customer.NewCustomerServer(custAPI, custRouter, &custCfg.Connection, &custLog)
		if err := server.Start(ctx); err != nil {
			custLog.Error().Err(err).Msg("CustomerBot Server failed")
		}
	}()

	// --- 6. Start Moderator Bot Server ---
	go func() {
		defer o.wg.Done()
		modClient.SetMenuCommands(ctx, 0, true) // admin menu

		// This server will poll and PUBLISH to the bus
		server := moderator.NewModeratorServer(modAPI, &modCfg.Connection, o.bus, &modLog)

		if err := server.Start(ctx); err != nil {
			modLog.Error().Err(err).Msg("ModeratorBot Server failed")
		}
	}()

	o.wg.Wait() // Wait for both goroutines to finish
	return nil
}
