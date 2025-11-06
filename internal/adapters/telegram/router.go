package telegram

import (
	"AsaExchange/internal/core/domain"
	"AsaExchange/internal/core/ports"
	"context"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/rs/zerolog"
)

// Router is the "Bot Facade." It holds all "plugins"
// and routes incoming updates to the correct handler.
type Router struct {
	log              zerolog.Logger
	userRepo         ports.UserRepository
	botClient        ports.BotClientPort
	commandHandlers  map[string]ports.CommandHandler
	callbackHandlers map[string]ports.CallbackHandler
	stateHandlers    map[domain.UserState]ports.StateHandler
}

// NewRouter creates a new bot facade/router.
func NewRouter(
	userRepo ports.UserRepository,
	botClient ports.BotClientPort,
	baseLogger *zerolog.Logger,
) *Router {
	return &Router{
		log:              baseLogger.With().Str("component", "tg_router").Logger(),
		userRepo:         userRepo,
		botClient:        botClient,
		commandHandlers:  make(map[string]ports.CommandHandler),
		callbackHandlers: make(map[string]ports.CallbackHandler),
		stateHandlers:    make(map[domain.UserState]ports.StateHandler),
	}
}

// RegisterCommandHandler adds a "plugin" to the router.
func (r *Router) RegisterCommandHandler(handler ports.CommandHandler) {
	cmd := handler.Command()
	r.commandHandlers[cmd] = handler
	r.log.Info().Str("command", cmd).Msg("Registered new command handler")
}

// RegisterCallbackHandler adds a "plugin" to the router.
func (r *Router) RegisterCallbackHandler(handler ports.CallbackHandler) {
	prefix := handler.Prefix()
	r.callbackHandlers[prefix] = handler
	r.log.Info().Str("prefix", prefix).Msg("Registered new callback handler")
}

// RegisterStateHandler adds a "plugin" for a specific user state
func (r *Router) RegisterStateHandler(handler ports.StateHandler) {
	state := handler.State()
	r.stateHandlers[state] = handler
	r.log.Info().Str("state", string(state)).Msg("Registered new state handler")
}

// HandleUpdate is the main entry point for a new update from Telegram.
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

	// 3. Route commands first
	if botUpdate.Command != "" {
		if handler, ok := r.commandHandlers[botUpdate.Command]; ok {
			ctxLogger.Info().Str("handler", botUpdate.Command).Msg("Routing to command handler")
			if err := handler.Handle(ctx, botUpdate); err != nil {
				ctxLogger.Error().Err(err).Msg("Command handler failed")
			}
			return
		}
		// Don't warn if it's an unknown command, just fall through to state handling
	}

	// 4. Route callbacks next
	if botUpdate.CallbackData != nil {
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

	// 5. --- STATE-BASED ROUTING ---
	// If it's not a command or callback, it's text.
	// We must check the user's state.
	user, err := r.userRepo.GetByTelegramID(ctx, botUpdate.UserID)
	if err != nil {
		ctxLogger.Error().Err(err).Msg("Failed to get user for state handling")
		r.botClient.SendMessage(ctx, ports.SendMessageParams{
			ChatID: botUpdate.ChatID,
			Text:   "An internal error occurred.",
		})
		return
	}

	if user == nil {
		// User sent text without ever typing /start
		r.botClient.SendMessage(ctx, ports.SendMessageParams{
			ChatID: botUpdate.ChatID,
			Text:   "Please type /start to begin\\.",
			ParseMode: "MarkdownV2",
		})
		return
	}

	// Check if we have a handler for the user's current state
	if handler, ok := r.stateHandlers[user.State]; ok {
		ctxLogger.Info().Str("state", string(user.State)).Msg("Routing to state handler")
		if err := handler.Handle(ctx, botUpdate, user); err != nil {
			ctxLogger.Error().Err(err).Msg("State handler failed")
		}
		return
	}

	// If we're here, it's an unhandled text message
	ctxLogger.Info().Str("text", botUpdate.Text).Msg("Received unhandled text message (no state)")
	// Optionally, send a "don't understand" message
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
