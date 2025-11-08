package moderator

import (
	"AsaExchange/internal/core/ports"
	"AsaExchange/internal/shared/config"
	"context"
	"fmt"
	"net/http"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/rs/zerolog"
)

// ModeratorServer is responsible for running the moderator bot
type ModeratorServer struct {
	api *tgbotapi.BotAPI
	cfg *config.BotConnectionConfig
	bus ports.EventBus
	log zerolog.Logger
}

// NewModeratorServer creates a new server instance
func NewModeratorServer(
	api *tgbotapi.BotAPI,
	cfg *config.BotConnectionConfig,
	bus ports.EventBus,
	baseLogger *zerolog.Logger,
) *ModeratorServer {
	return &ModeratorServer{
		api: api,
		cfg: cfg,
		bus: bus,
		log: baseLogger.With().Str("component", "moderator_server").Logger(),
	}
}

// Start begins the bot server based on the config mode
func (s *ModeratorServer) Start(ctx context.Context) error {
	s.log.Info().Str("mode", s.cfg.Mode).Msg("Starting moderator server...")

	switch s.cfg.Mode {
	case "polling":
		return s.startPolling(ctx)
	case "webhook":
		return s.startWebhook(ctx)
	default:
		return fmt.Errorf("unknown bot mode: %s", s.cfg.Mode)
	}
}

// startPolling
// This is now a "dumb" poller that publishes to the event bus.
func (s *ModeratorServer) startPolling(ctx context.Context) error {
	s.log.Info().Int("workers", s.cfg.Polling.WorkerPoolSize).Msg("Starting bot in POLLING mode")

	// 1. Clear any existing webhook
	deleteWebhookConfig := tgbotapi.DeleteWebhookConfig{
		DropPendingUpdates: false,
	}
	if _, err := s.api.Request(deleteWebhookConfig); err != nil {
		s.log.Warn().Err(err).Msg("Failed to delete webhook (continuing anyway)")
	} else {
		s.log.Info().Msg("Webhook deleted successfully")
	}

	// 2. Set up the update channel to listen for ALL types
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	// We are now listening for messages, callbacks, AND channel posts
	u.AllowedUpdates = []string{"message", "callback_query", "channel_post"}

	updates := s.api.GetUpdatesChan(u)

	s.log.Info().Msg("Polling update listener started")

	// 3. Main loop: Poll and Publish
	for {
		select {
		case <-ctx.Done(): // Shutdown signal received
			s.api.StopReceivingUpdates()
			s.log.Info().Msg("Polling stopped gracefully")
			return nil
		case update := <-updates:
			// We don't process the update. We just publish it to the bus.
			s.publishUpdateToBus(ctx, update)
		}
	}
}

// startWebhook
// This is now a "dumb" server that publishes to the event bus.
func (s *ModeratorServer) startWebhook(ctx context.Context) error {
	s.log.Info().
		Int("port", s.cfg.Webhook.ListenPort).
		Msg("Starting bot in WEBHOOK mode")

	// 1. Set the webhook
	webhookURL := fmt.Sprintf("%s/webhook/%s", s.cfg.Webhook.URL, s.api.Token)
	s.log.Info().Str("url", webhookURL).Msg("Setting webhook...")

	wh, err := tgbotapi.NewWebhook(webhookURL)
	if err != nil {
		s.log.Error().Err(err).Msg("Failed to create webhook config")
		return err
	}
	if _, err = s.api.Request(wh); err != nil {
		s.log.Error().Err(err).Msg("Failed to set webhook")
		return err
	}
	info, err := s.api.GetWebhookInfo()
	if err != nil {
		s.log.Error().Err(err).Msg("Failed to get webhook info")
		return err
	}
	if info.LastErrorDate != 0 {
		s.log.Error().
			Str("error_message", info.LastErrorMessage).
			Msg("Telegram webhook has a last error")
	} else {
		s.log.Info().Msg("Webhook set successfully, no last error")
	}

	// 2. Get update channel
	updates := s.api.ListenForWebhook("/webhook/" + s.api.Token)

	// 3. Start HTTP server
	listenAddr := fmt.Sprintf("127.0.0.1:%d", s.cfg.Webhook.ListenPort)
	s.log.Info().Str("addr", listenAddr).Msg("Starting HTTP server for webhook")

	httpServer := &http.Server{Addr: listenAddr}
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.log.Error().Err(err).Msg("Webhook HTTP server failed")
		}
	}()

	s.log.Info().Msg("Webhook update listener started")

	// 4. Main loop: Listen and Publish
	for {
		select {
		case <-ctx.Done():
			s.log.Info().Msg("Shutting down HTTP server...")
			if err := httpServer.Shutdown(context.Background()); err != nil {
				s.log.Error().Err(err).Msg("HTTP server shutdown error")
			}
			s.log.Info().Msg("Webhook server stopped gracefully")
			return nil
		case update := <-updates:
			s.publishUpdateToBus(ctx, update)
		}
	}
}

// publishUpdateToBus inspects the update and publishes it to the correct topic.
func (s *ModeratorServer) publishUpdateToBus(ctx context.Context, update tgbotapi.Update) {
	if update.ChannelPost != nil {
		s.bus.Publish(ctx, "telegram:mod:channel_post", update)
	} else if update.Message != nil {
		s.bus.Publish(ctx, "telegram:mod:message", update)
	} else if update.CallbackQuery != nil {
		s.bus.Publish(ctx, "telegram:mod:callback_query", update)
	}
}
