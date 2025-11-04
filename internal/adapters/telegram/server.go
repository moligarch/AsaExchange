package telegram

import (
	"AsaExchange/internal/shared/config"
	"context"
	"fmt"
	"net/http" // <-- IMPORTED
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/rs/zerolog"
)

// BotServer is responsible for running the bot (polling or webhook)
type BotServer struct {
	api    *tgbotapi.BotAPI
	router *Router
	cfg    *config.BotConfig
	log    zerolog.Logger
}

// NewBotServer creates a new server instance
func NewBotServer(
	api *tgbotapi.BotAPI,
	router *Router,
	cfg *config.BotConfig,
	baseLogger *zerolog.Logger,
) *BotServer {
	return &BotServer{
		api:    api,
		router: router,
		cfg:    cfg,
		log:    baseLogger.With().Str("component", "bot_server").Logger(),
	}
}

// Start begins the bot server based on the config mode
func (s *BotServer) Start(ctx context.Context) error {
	s.log.Info().Str("mode", s.cfg.Mode).Msg("Starting bot server...")

	switch s.cfg.Mode {
	case "polling":
		// startPolling will block until the context is cancelled
		return s.startPolling(ctx)
	case "webhook":
		// startWebhook will block until the context is cancelled
		return s.startWebhook(ctx)
	default:
		return fmt.Errorf("unknown bot mode: %s", s.cfg.Mode)
	}
}

// startPolling starts the bot in long polling mode with a worker pool
func (s *BotServer) startPolling(ctx context.Context) error {
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

	// 2. Create the channel for updates
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := s.api.GetUpdatesChan(u)

	// 3. Create the job channel
	jobs := make(chan tgbotapi.Update, 100) // Buffered channel

	// 4. Start the worker pool
	var wg sync.WaitGroup
	for w := 1; w <= s.cfg.Polling.WorkerPoolSize; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			// Get the router's logger and add worker ID
			log := s.router.log.With().Int("worker_id", id).Logger()
			log.Info().Msg("Starting polling worker")
			for {
				select {
				case <-ctx.Done(): // Context cancelled
					log.Info().Msg("Stopping polling worker (context done)")
					return
				case job, ok := <-jobs:
					if !ok {
						log.Info().Msg("Stopping polling worker (channel closed)")
						return
					}
					// Process the update
					s.router.HandleUpdate(context.Background(), &job)
				}
			}
		}(w)
	}

	s.log.Info().Msg("Polling update listener started")

	// 5. Main loop: Listen for updates and dispatch jobs
	for {
		select {
		case <-ctx.Done(): // Shutdown signal received
			close(jobs)                  // Close the jobs channel to signal workers
			s.api.StopReceivingUpdates() // Stop the bot API
			wg.Wait()                    // Wait for all workers to finish
			s.log.Info().Msg("Polling stopped gracefully")
			return nil // Return nil on graceful shutdown
		case update := <-updates:
			// Send the update to the worker pool
			jobs <- update
		}
	}
}

// startWebhook starts the bot in webhook mode (for production)
func (s *BotServer) startWebhook(ctx context.Context) error {
	s.log.Info().
		Int("port", s.cfg.Webhook.ListenPort).
		Int("workers", s.cfg.Polling.WorkerPoolSize). // We reuse the worker pool size
		Msg("Starting bot in WEBHOOK mode")

	// 1. Set the webhook
	webhookURL := fmt.Sprintf("%s/webhook/%s", s.cfg.Webhook.URL, s.api.Token)
	s.log.Info().Str("url", webhookURL).Msg("Setting webhook...")

	wh, err := tgbotapi.NewWebhook(webhookURL)
	if err != nil {
		s.log.Error().Err(err).Msg("Failed to create webhook config")
		return err
	}

	_, err = s.api.Request(wh)
	if err != nil {
		s.log.Error().Err(err).Msg("Failed to set webhook")
		return err
	}

	// 2. Add GetWebhookInfo check
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

	// 3. Get the update channel from the bot library
	// This sets up the http.DefaultServeMux
	updates := s.api.ListenForWebhook("/webhook/" + s.api.Token)

	// 4. Start the HTTP server in a goroutine
	// We use ListenAndServe, not ListenAndServeTLS,
	// assuming a reverse proxy (Nginx, Caddy) is handling SSL.
	listenAddr := fmt.Sprintf("127.0.0.1:%d", s.cfg.Webhook.ListenPort)
	s.log.Info().Str("addr", listenAddr).Msg("Starting HTTP server for webhook")

	httpServer := &http.Server{Addr: listenAddr}
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.log.Error().Err(err).Msg("Webhook HTTP server failed")
		}
	}()

	// 5. Start the worker pool (identical to polling)
	jobs := make(chan tgbotapi.Update, 100)
	var wg sync.WaitGroup
	for w := 1; w <= s.cfg.Polling.WorkerPoolSize; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			log := s.router.log.With().Int("worker_id", id).Logger()
			log.Info().Msg("Starting webhook worker")
			for {
				select {
				case <-ctx.Done():
					log.Info().Msg("Stopping webhook worker (context done)")
					return
				case job, ok := <-jobs:
					if !ok {
						log.Info().Msg("Stopping webhook worker (channel closed)")
						return
					}
					s.router.HandleUpdate(context.Background(), &job)
				}
			}
		}(w)
	}

	// 6. Main loop: Listen for updates and dispatch jobs
	s.log.Info().Msg("Webhook update listener started")
	for {
		select {
		case <-ctx.Done(): // Shutdown signal received
			close(jobs) // Close the jobs channel

			// Shut down the HTTP server
			s.log.Info().Msg("Shutting down HTTP server...")
			if err := httpServer.Shutdown(context.Background()); err != nil {
				s.log.Error().Err(err).Msg("HTTP server shutdown error")
			}

			wg.Wait() // Wait for workers
			s.log.Info().Msg("Webhook server stopped gracefully")
			return nil
		case update := <-updates:
			// Send the update to the worker pool
			jobs <- update
		}
	}
}
