package moderator

import (
	// <-- NEW IMPORT
	"AsaExchange/internal/core/ports"
	"context"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/rs/zerolog"
)

// ModeratorRouter holds all logic for the admin bot
type ModeratorRouter struct {
	log              zerolog.Logger
	userRepo         ports.UserRepository
	botClient        ports.BotClientPort
	commandHandlers  map[string]ports.CommandHandler
	callbackHandlers map[string]ports.CallbackHandler
	messageHandler   ports.MessageHandler // <-- ADDED
}

// NewModeratorRouter creates a new admin bot router
func NewModeratorRouter(
	userRepo ports.UserRepository,
	botClient ports.BotClientPort,
	bus ports.EventBus,
	baseLogger *zerolog.Logger,
) *ModeratorRouter {
	router := &ModeratorRouter{
		log:              baseLogger.With().Str("component", "moderator_router").Logger(),
		userRepo:         userRepo,
		botClient:        botClient,
		commandHandlers:  make(map[string]ports.CommandHandler),
		callbackHandlers: make(map[string]ports.CallbackHandler),
		// messageHandler is nil by default
	}

	// Subscribe to the event bus topics
	bus.Subscribe("telegram:mod:message", router.handleMessage)
	bus.Subscribe("telegram:mod:callback_query", router.handleCallbackQuery)

	return router
}

// RegisterCommandHandler (UNCHANGED)
func (r *ModeratorRouter) RegisterCommandHandler(handler ports.CommandHandler) {
	cmd := handler.Command()
	r.commandHandlers[cmd] = handler
	r.log.Info().Str("command", cmd).Msg("Registered new moderator command")
}

// RegisterCallbackHandler (UNCHANGED)
func (r *ModeratorRouter) RegisterCallbackHandler(handler ports.CallbackHandler) {
	prefix := handler.Prefix()
	r.callbackHandlers[prefix] = handler
	r.log.Info().Str("prefix", prefix).Msg("Registered new moderator callback")
}

// SetMessageHandler registers the single, global message handler
func (r *ModeratorRouter) SetMessageHandler(handler ports.MessageHandler) {
	r.messageHandler = handler
}

// --- END NEW METHOD ---

// This method is called by the EventBus
func (r *ModeratorRouter) handleMessage(ctx context.Context, event ports.Event) error {
	update, ok := event.Data.(tgbotapi.Update)
	if !ok || update.Message == nil {
		r.log.Error().Msg("Received bad message event")
		return nil // Don't retry
	}

	botUpdate, isSupported := r.parseUpdate(update)
	if !isSupported {
		return nil // Not an update we care about
	}

	// Add logger context
	ctxLogger := r.log.With().
		Int64("user_id", botUpdate.UserID).
		Int64("chat_id", botUpdate.ChatID).
		Logger()
	ctx = ctxLogger.WithContext(ctx)

	user, err := r.userRepo.GetByTelegramID(ctx, botUpdate.UserID)
	if err != nil {
		ctxLogger.Error().Err(err).Msg("Failed to get user for security check")
		return err // Let bus log the error
	}

	if user == nil || !user.IsModerator {
		ctxLogger.Warn().Msg("Unauthorized user tried to access moderator bot")
		return nil // Don't retry
	}

	// Route command
	if botUpdate.Command != "" {
		if handler, ok := r.commandHandlers[botUpdate.Command]; ok {
			ctxLogger.Info().Str("handler", botUpdate.Command).Msg("Routing to mod command handler")
			if err := handler.Handle(ctx, botUpdate); err != nil {
				// The handler will log its own error
				return err
			}
			return nil
		}
	}

	// Route to MessageHandler
	// If it's not a command, check for a message handler
	if r.messageHandler != nil {
		if err := r.messageHandler.Handle(ctx, botUpdate, user); err != nil {
			ctxLogger.Error().Err(err).Msg("Mod message handler failed")
			return err
		}
		return nil
	}
	// --- END NEW ---

	ctxLogger.Warn().Msg("Moderator bot received unhandled message")
	return nil
}

// This method is called by the EventBus (UNCHANGED)
func (r *ModeratorRouter) handleCallbackQuery(ctx context.Context, event ports.Event) error {
	update, ok := event.Data.(tgbotapi.Update)
	if !ok || update.CallbackQuery == nil {
		r.log.Error().Msg("Received bad callback_query event")
		return nil // Don't retry
	}

	botUpdate, isSupported := r.parseUpdate(update)
	if !isSupported {
		return nil // Not an update we care about
	}

	// Add logger context
	ctxLogger := r.log.With().
		Int64("user_id", botUpdate.UserID).
		Int64("chat_id", botUpdate.ChatID).
		Logger()
	ctx = ctxLogger.WithContext(ctx)

	user, err := r.userRepo.GetByTelegramID(ctx, botUpdate.UserID)
	if err != nil {
		ctxLogger.Error().Err(err).Msg("Failed to get user for security check")
		return err // Let bus log the error
	}

	if user == nil || !user.IsModerator {
		ctxLogger.Warn().Msg("Unauthorized user tried to access moderator bot")
		return nil // Don't retry
	}

	// Route callback
	if botUpdate.CallbackData != nil {
		for prefix, handler := range r.callbackHandlers {
			if strings.HasPrefix(*botUpdate.CallbackData, prefix) {
				ctxLogger.Info().Str("handler", prefix).Str("data", *botUpdate.CallbackData).Msg("Routing to callback handler")
				if err := handler.Handle(ctx, botUpdate, user); err != nil {
					// The handler will log its own error
					return err
				}
				return nil
			}
		}
		ctxLogger.Warn().Str("data", *botUpdate.CallbackData).Msg("No callback handler found")
	}

	return nil
}

// parseUpdate (UNCHANGED)
func (r *ModeratorRouter) parseUpdate(update tgbotapi.Update) (*ports.BotUpdate, bool) {
	if update.CallbackQuery != nil {
		cb := update.CallbackQuery
		return &ports.BotUpdate{
			MessageID:       cb.Message.MessageID,
			ChatID:          cb.Message.Chat.ID,
			UserID:          cb.From.ID,
			CallbackQueryID: cb.ID,
			CallbackData:    &cb.Data,
		}, true
	}

	if update.Message != nil {
		msg := update.Message
		return &ports.BotUpdate{
			MessageID: msg.MessageID,
			ChatID:    msg.Chat.ID,
			UserID:    msg.From.ID,
			Text:      msg.Text,
			Command:   msg.Command(),
		}, true
	}

	// We ignore channel posts here
	return nil, false
}
