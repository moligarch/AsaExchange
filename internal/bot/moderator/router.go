package moderator

import (
	"AsaExchange/internal/core/ports"
	"context"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/rs/zerolog"
	// "strings"
)

// ModeratorRouter holds all logic for the admin bot
type ModeratorRouter struct {
	log             zerolog.Logger
	userRepo        ports.UserRepository
	botClient       ports.BotClientPort
	commandHandlers map[string]ports.CommandHandler
	// ... callbackHandlers, messageHandler ...
}

// NewModeratorRouter creates a new admin bot router
func NewModeratorRouter(
	userRepo ports.UserRepository,
	botClient ports.BotClientPort,
	baseLogger *zerolog.Logger,
) *ModeratorRouter {
	return &ModeratorRouter{
		log:             baseLogger.With().Str("component", "moderator_router").Logger(),
		userRepo:        userRepo,
		botClient:       botClient,
		commandHandlers: make(map[string]ports.CommandHandler),
	}
}

// RegisterCommandHandler (UNCHANGED)
func (r *ModeratorRouter) RegisterCommandHandler(handler ports.CommandHandler) {
	cmd := handler.Command()
	r.commandHandlers[cmd] = handler
	r.log.Info().Str("command", cmd).Msg("Registered new moderator command")
}

// ... RegisterCallbackHandler, SetMessageHandler ...

// HandleUpdate is the main entry point for the admin bot
func (r *ModeratorRouter) HandleUpdate(ctx context.Context, update *tgbotapi.Update) {
	// 1. Convert to our generic BotUpdate
	botUpdate, isSupported := r.parseUpdate(update)
	if !isSupported {
		r.log.Warn().Interface("update", update).Msg("Received unsupported update type")
		return
	}

	// 2. Add logger context
	ctxLogger := r.log.With().
		Int64("user_id", botUpdate.UserID).
		Int64("chat_id", botUpdate.ChatID).
		Logger()
	ctx = ctxLogger.WithContext(ctx)

	// 3. --- CRITICAL SECURITY CHECK ---
	user, err := r.userRepo.GetByTelegramID(ctx, botUpdate.UserID)
	if err != nil {
		ctxLogger.Error().Err(err).Msg("Failed to get user for security check")
		return
	}

	if user == nil || !user.IsModerator {
		ctxLogger.Warn().Msg("Unauthorized user tried to access moderator bot")
		// We can optionally send a "permission denied" message
		// r.botClient.SendMessage(ctx, ports.SendMessageParams{...})
		return
	}
	// --- END SECURITY CHECK ---

	// 4. Route commands
	if botUpdate.Command != "" {
		if handler, ok := r.commandHandlers[botUpdate.Command]; ok {
			ctxLogger.Info().Str("handler", botUpdate.Command).Msg("Routing to mod command handler")
			if err := handler.Handle(ctx, botUpdate); err != nil {
				ctxLogger.Error().Err(err).Msg("Mod command handler failed")
			}
			return
		}
	}

	// ... (Handle callbacks and messages later) ...

	ctxLogger.Warn().Msg("Moderator bot received unhandled update")
}

// parseUpdate (a copy of the customer one for now)
func (r *ModeratorRouter) parseUpdate(update *tgbotapi.Update) (*ports.BotUpdate, bool) {
	if update.CallbackQuery != nil {
		cb := update.CallbackQuery
		return &ports.BotUpdate{
			MessageID:    cb.Message.MessageID,
			ChatID:       cb.Message.Chat.ID,
			UserID:       cb.From.ID,
			CallbackData: &cb.Data,
		}, true
	}

	if update.Message != nil {
		msg := update.Message

		var contactInfo *ports.ContactInfo
		if msg.Contact != nil {
			contactInfo = &ports.ContactInfo{
				PhoneNumber: msg.Contact.PhoneNumber,
				UserID:      msg.Contact.UserID,
			}
		}

		var photoInfo *ports.PhotoInfo
		if len(msg.Photo) > 0 {
			bestPhoto := msg.Photo[len(msg.Photo)-1]
			photoInfo = &ports.PhotoInfo{
				FileID:   bestPhoto.FileID,
				FileSize: bestPhoto.FileSize,
			}
		}

		return &ports.BotUpdate{
			MessageID: msg.MessageID,
			ChatID:    msg.Chat.ID,
			UserID:    msg.From.ID,
			Text:      msg.Text,
			Command:   msg.Command(),
			Contact:   contactInfo,
			Photo:     photoInfo,
		}, true
	}

	return nil, false
}
