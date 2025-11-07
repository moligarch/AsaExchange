package moderator

import (
	"AsaExchange/internal/shared/config"
	"context"
	"fmt"
	"net/http"
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/rs/zerolog"
)

// ModeratorServer is responsible for running the moderator bot
type ModeratorServer struct {
	api    *tgbotapi.BotAPI
	router *ModeratorRouter
	cfg    *config.BotConnectionConfig // Use the same connection config struct
	log    zerolog.Logger
}

// NewModeratorServer creates a new server instance
func NewModeratorServer(
	api *tgbotapi.BotAPI,
	router *ModeratorRouter,
	cfg *config.BotConnectionConfig,
	baseLogger *zerolog.Logger,
) *ModeratorServer {
	return &ModeratorServer{
		api:    api,
		router: router,
		cfg:    cfg,
		log:    baseLogger.With().Str("component", "moderator_server").Logger(),
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

// startPolling (Identical to customer server)
func (s *ModeratorServer) startPolling(ctx context.Context) error {
	s.log.Info().Int("workers", s.cfg.Polling.WorkerPoolSize).Msg("Starting bot in POLLING mode")

	deleteWebhookConfig := tgbotapi.DeleteWebhookConfig{
		DropPendingUpdates: false,
	}
	if _, err := s.api.Request(deleteWebhookConfig); err != nil {
		s.log.Warn().Err(err).Msg("Failed to delete webhook (continuing anyway)")
	} else {
		s.log.Info().Msg("Webhook deleted successfully")
	}

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := s.api.GetUpdatesChan(u)

	jobs := make(chan tgbotapi.Update, 100)

	var wg sync.WaitGroup
	for w := 1; w <= s.cfg.Polling.WorkerPoolSize; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			log := s.router.log.With().Int("worker_id", id).Logger()
			log.Info().Msg("Starting polling worker")
			for {
				select {
				case <-ctx.Done():
					log.Info().Msg("Stopping polling worker (context done)")
					return
				case job, ok := <-jobs:
					if !ok {
						log.Info().Msg("Stopping polling worker (channel closed)")
						return
					}
					s.router.HandleUpdate(context.Background(), &job)
				}
			}
		}(w)
	}

	s.log.Info().Msg("Polling update listener started")

	for {
		select {
		case <-ctx.Done():
			close(jobs)
			s.api.StopReceivingUpdates()
			wg.Wait()
			s.log.Info().Msg("Polling stopped gracefully")
			return nil
		case update := <-updates:
			jobs <- update
		}
	}
}

// startWebhook (Identical to customer server)
func (s *ModeratorServer) startWebhook(ctx context.Context) error {
	s.log.Info().
		Int("port", s.cfg.Webhook.ListenPort).
		Int("workers", s.cfg.Polling.WorkerPoolSize).
		Msg("Starting bot in WEBHOOK mode")

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

	updates := s.api.ListenForWebhook("/webhook/" + s.api.Token)

	listenAddr := fmt.Sprintf("127.0.0.1:%d", s.cfg.Webhook.ListenPort)
	s.log.Info().Str("addr", listenAddr).Msg("Starting HTTP server for webhook")

	httpServer := &http.Server{Addr: listenAddr}
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.log.Error().Err(err).Msg("Webhook HTTP server failed")
		}
	}()

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

	s.log.Info().Msg("Webhook update listener started")
	for {
		select {
		case <-ctx.Done():
			close(jobs)

			s.log.Info().Msg("Shutting down HTTP server...")
			if err := httpServer.Shutdown(context.Background()); err != nil {
				s.log.Error().Err(err).Msg("HTTP server shutdown error")
			}

			wg.Wait()
			s.log.Info().Msg("Webhook server stopped gracefully")
			return nil
		case update := <-updates:
			jobs <- update
		}
	}
}
