package telegram

import (
	"AsaExchange/internal/core/ports"
	"context"
	"errors"
	"fmt"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// telegramQueue implements the VerificationQueue interface
// It publishes to Telegram and subscribes to the EventBus
type telegramQueue struct {
	customerBot ports.BotClientPort // Used to Publish
	channelID   int64
	bus         ports.EventBus
	log         zerolog.Logger
}

// NewTelegramQueue creates our MVP "queue"
func NewTelegramQueue(
	customerBot ports.BotClientPort,
	channelID int64,
	bus ports.EventBus,
	baseLogger *zerolog.Logger,
) ports.VerificationQueue {
	return &telegramQueue{
		customerBot: customerBot,
		channelID:   channelID,
		bus:         bus,
		log:         baseLogger.With().Str("component", "telegram_queue").Logger(),
	}
}

// Publish sends the photo+caption to the private channel
func (t *telegramQueue) Publish(ctx context.Context, event ports.NewVerificationEvent) (string, error) {
	params := ports.SendPhotoParams{
		ChatID:    t.channelID,
		File:      tgbotapi.FileID(event.FileID),
		Caption:   event.Caption,
		ParseMode: "", // Plain text
	}

	// Use the CustomerBot client to send the photo
	messageID, err := t.customerBot.SendPhoto(ctx, params)
	if err != nil {
		t.log.Error().Err(err).Msg("Failed to publish photo to queue channel")
		return "", err
	}

	// The storage reference IS the message ID
	return fmt.Sprintf("%d", messageID), nil
}

// Subscribe registers the queue's handler with the event bus.
// It no longer polls.
func (t *telegramQueue) Subscribe(ctx context.Context, handler func(event ports.NewVerificationEvent) error) {
	// Register our internal method as the handler for this topic
	t.bus.Subscribe("telegram:mod:channel_post", t.handleChannelPost(handler))
	t.log.Info().Int64("channel_id", t.channelID).Msg("Subscribed to 'telegram:mod:channel_post' topic")
}

// handleChannelPost is the internal function that the EventBus will call.
// It wraps the final handler with our parsing logic.
func (t *telegramQueue) handleChannelPost(handler func(event ports.NewVerificationEvent) error) ports.EventHandler {
	// The event bus calls this function
	return func(ctx context.Context, event ports.Event) error {
		update, ok := event.Data.(tgbotapi.Update)
		if !ok || update.ChannelPost == nil {
			t.log.Error().Msg("Received bad channel_post event from bus")
			return nil // Don't retry
		}

		// We only care about channel posts in our specific channel
		if update.ChannelPost.Chat.ID != t.channelID {
			return nil // Ignore this post
		}

		msg := update.ChannelPost

		// We only care about photo messages
		if msg.Photo == nil {
			t.log.Warn().Int("msg_id", msg.MessageID).Msg("Received non-photo message in upload channel")
			return nil
		}

		t.log.Info().Int("message_id", msg.MessageID).Msg("Received new event from queue")

		// Parse the UserID from the caption
		userID, err := t.parseUserIDFromCaption(msg.Caption)
		if err != nil {
			t.log.Error().Err(err).Int("msg_id", msg.MessageID).Msg("Failed to parse UserID from caption")
			return nil
		}

		bestPhoto := msg.Photo[len(msg.Photo)-1]

		// Re-create the event
		// The FileID is now the one the *Moderator Bot* can use
		newEvent := ports.NewVerificationEvent{
			UserID:  userID,
			FileID:  bestPhoto.FileID,
			Caption: msg.Caption,
		}

		// Call the final handler (the forwarding_handler)
		// The handler's signature is func(event ports.NewVerificationEvent) error
		if err := handler(newEvent); err != nil {
			t.log.Error().Err(err).Str("user_id", newEvent.UserID.String()).Msg("Queue handler failed to process event")
			return err
		}

		return nil
	}
}

// parseUserIDFromCaption finds the UserID in the caption.
func (t *telegramQueue) parseUserIDFromCaption(caption string) (uuid.UUID, error) {
	lines := strings.Split(caption, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "UserID: ") {
			idStr := strings.TrimPrefix(line, "UserID: ")
			id, err := uuid.Parse(idStr)
			if err != nil {
				return uuid.Nil, err
			}
			return id, nil
		}
	}
	return uuid.Nil, errors.New("UserID not found in caption")
}
