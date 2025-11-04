package telegram

import (
	"AsaExchange/internal/core/ports"
	"context"
	"strings" // <-- Make sure this is imported

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/rs/zerolog"
)

// Router is the "Bot Facade." It holds all "plugins"
// and routes incoming updates to the correct handler.
type Router struct {
	log              zerolog.Logger
	userRepo         ports.UserRepository
	commandHandlers  map[string]ports.CommandHandler
	callbackHandlers map[string]ports.CallbackHandler
}

// NewRouter creates a new bot facade/router.
func NewRouter(userRepo ports.UserRepository, baseLogger *zerolog.Logger) *Router {
	return &Router{
		log:              baseLogger.With().Str("component", "tg_router").Logger(),
		userRepo:         userRepo,
		commandHandlers:  make(map[string]ports.CommandHandler),
		callbackHandlers: make(map[string]ports.CallbackHandler),
	}
}

// RegisterCommandHandler adds a "plugin" to the router.
// (This version is correct)
func (r *Router) RegisterCommandHandler(handler ports.CommandHandler) {
	cmd := handler.Command()
	r.commandHandlers[cmd] = handler
	r.log.Info().Str("command", cmd).Msg("Registered new command handler")
}

// RegisterCallbackHandler adds a "plugin" to the router.
// (This version is correct)
func (r *Router) RegisterCallbackHandler(handler ports.CallbackHandler) {
	prefix := handler.Prefix()
	r.callbackHandlers[prefix] = handler
	r.log.Info().Str("prefix", prefix).Msg("Registered new callback handler")
}

// HandleUpdate is the main entry point for a new update from Telegram.
// (This logic is correct and UNCHANGED)
func (r *Router) HandleUpdate(ctx context.Context, update *tgbotapi.Update) {
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

	// 3. Route the update
	if botUpdate.CallbackData != nil {
		// This is a callback
		for prefix, handler := range r.callbackHandlers {
			if strings.HasPrefix(*botUpdate.CallbackData, prefix) {
				ctxLogger.Info().Str("handler", prefix).Str("data", *botUpdate.CallbackData).Msg("Routing to callback handler")
				if err := handler.Handle(ctx, botUpdate); err != nil {
					ctxLogger.Error().Err(err).Msg("Callback handler failed")
				}
				return
			}
		}
		ctxLogger.Warn().Str("data", *botUpdate.CallbackData).Msg("No callback handler found")
		return
	}

	if botUpdate.Command != "" {
		// This is a command
		if handler, ok := r.commandHandlers[botUpdate.Command]; ok {
			ctxLogger.Info().Str("handler", botUpdate.Command).Msg("Routing to command handler")
			if err := handler.Handle(ctx, botUpdate); err != nil {
				ctxLogger.Error().Err(err).Msg("Command handler failed")
			}
			return
		}
		ctxLogger.Warn().Str("command", botUpdate.Command).Msg("No command handler found")
		return
	}

	ctxLogger.Info().Str("text", botUpdate.Text).Msg("Received unhandled text message")
}

// parseUpdate converts a tgbotapi.Update into our internal, simplified struct.
func (r *Router) parseUpdate(update *tgbotapi.Update) (*ports.BotUpdate, bool) {
	if update.CallbackQuery != nil {
		// This is a Callback
		cb := update.CallbackQuery
		return &ports.BotUpdate{
			MessageID:    cb.Message.MessageID,
			ChatID:       cb.Message.Chat.ID,
			UserID:       cb.From.ID,
			CallbackData: &cb.Data,
		}, true
	}

	if update.Message != nil {
		// This is a Message
		msg := update.Message
		return &ports.BotUpdate{
			MessageID: msg.MessageID,
			ChatID:    msg.Chat.ID,
			UserID:    msg.From.ID,
			Text:      msg.Text,
			Command:   msg.Command(),
		}, true
	}

	return nil, false // Unsupported update
}
