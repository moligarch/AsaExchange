package handlers

import (
	"AsaExchange/internal/bot/messages"
	"AsaExchange/internal/core/domain"
	"AsaExchange/internal/core/ports"
	"context"

	"github.com/rs/zerolog"
)

// NotificationHandler listens for internal events (from the EventBus)
// and sends messages to users via the Customer Bot.
type NotificationHandler struct {
	log        zerolog.Logger
	custClient ports.BotClientPort
	userRepo   ports.UserRepository // Added for fetching fresh user data if needed
}

// NewNotificationHandler creates a new handler for sending user notifications.
// It is NOT a registered router/message handler; it's a system component.
func NewNotificationHandler(
	custClient ports.BotClientPort,
	userRepo ports.UserRepository,
	baseLogger *zerolog.Logger,
) *NotificationHandler {
	return &NotificationHandler{
		log:        baseLogger.With().Str("component", "notification_handler").Logger(),
		custClient: custClient,
		userRepo:   userRepo,
	}
}

// HandleUserApproved is an EventHandler for the "user:approved" topic.
func (h *NotificationHandler) HandleUserApproved(ctx context.Context, event ports.Event) error {
	user, ok := event.Data.(*domain.User)
	if !ok {
		h.log.Error().Msg("Received invalid data for 'user:approved' event")
		return nil // Don't retry
	}

	log := h.log.With().Str("user_id", user.ID.String()).Logger()
	log.Info().Msg("Sending approval notification to user")

	msg := messages.NewBuilder(user.TelegramID).
		WithText(
			"🎉 Your account has been *approved*\\! You can now start using the exchange\\. Type /start to see your options\\.",
		).
		Build()

	if _, err := h.custClient.SendMessage(ctx, msg); err != nil {
		log.Error().Err(err).Msg("Failed to send approval notification")
		return err
	}
	return nil
}

// HandleUserRejected is an EventHandler for the "user:rejected" topic.
func (h *NotificationHandler) HandleUserRejected(ctx context.Context, event ports.Event) error {
	user, ok := event.Data.(*domain.User)
	if !ok {
		h.log.Error().Msg("Received invalid data for 'user:rejected' event")
		return nil // Don't retry
	}

	log := h.log.With().Str("user_id", user.ID.String()).Logger()
	log.Info().Msg("Sending rejection notification to user")

	msg := messages.NewBuilder(user.TelegramID).
		WithText(
			"Your identity verification was *rejected*\\. Please type /start to try the registration process again\\.",
		).
		Build()

	if _, err := h.custClient.SendMessage(ctx, msg); err != nil {
		log.Error().Err(err).Msg("Failed to send rejection notification")
		return err
	}
	return nil
}
