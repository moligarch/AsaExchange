package customer

import (
	"AsaExchange/internal/bot/messages"
	"AsaExchange/internal/core/ports"
	"context"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/rs/zerolog"
)

// Router is the "Bot Facade." It holds all "plugins"
// and routes incoming updates to the correct handler.
type CustomerRouter struct {
	log              zerolog.Logger
	userRepo         ports.UserRepository
	botClient        ports.BotClientPort
	commandHandlers  map[string]ports.CommandHandler
	callbackHandlers map[string]ports.CallbackHandler
	messageHandler   ports.MessageHandler
}

// NewRouter creates a new bot facade/router.
func NewCustomerRouter(
	userRepo ports.UserRepository,
	botClient ports.BotClientPort,
	baseLogger *zerolog.Logger,
) *CustomerRouter {
	return &CustomerRouter{
		log:              baseLogger.With().Str("component", "customer_router").Logger(),
		userRepo:         userRepo,
		botClient:        botClient,
		commandHandlers:  make(map[string]ports.CommandHandler),
		callbackHandlers: make(map[string]ports.CallbackHandler),
	}
}

// RegisterCommandHandler adds a "plugin" to the router.
func (r *CustomerRouter) RegisterCommandHandler(handler ports.CommandHandler) {
	cmd := handler.Command()
	r.commandHandlers[cmd] = handler
	r.log.Info().Str("command", cmd).Msg("Registered new command handler")
}

// RegisterCallbackHandler adds a "plugin" to the router.
func (r *CustomerRouter) RegisterCallbackHandler(handler ports.CallbackHandler) {
	prefix := handler.Prefix()
	r.callbackHandlers[prefix] = handler
	r.log.Info().Str("prefix", prefix).Msg("Registered new callback handler")
}

// SetTextHandler registers the single, global text handler
func (r *CustomerRouter) SetMessageHandler(handler ports.MessageHandler) {
	r.messageHandler = handler
}

// HandleUpdate is the main entry point for a new update from Telegram.
// If it's *anything* else (Text, Contact, Photo...), pass it to the message handler.
func (r *CustomerRouter) HandleUpdate(ctx context.Context, update *tgbotapi.Update) {
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

	// 3. Get user ONCE at the start
	// Both callbacks and messages need the user object.
	user, err := r.userRepo.GetByTelegramID(ctx, botUpdate.UserID)
	if err != nil {
		ctxLogger.Error().Err(err).Msg("Failed to get user for handling")
		r.botClient.SendMessage(ctx, ports.SendMessageParams{
			ChatID: botUpdate.ChatID,
			Text:   "An internal error occurred.",
		})
		return
	}

	// 4. Route commands first (they might create the user)
	if botUpdate.Command != "" {
		if handler, ok := r.commandHandlers[botUpdate.Command]; ok {
			ctxLogger.Info().Str("handler", botUpdate.Command).Msg("Routing to command handler")
			if err := handler.Handle(ctx, botUpdate); err != nil {
				ctxLogger.Error().Err(err).Msg("Command handler failed")
			}
			return
		}
	}

	// 5. Check for nil user *after* command check
	if user == nil {
		// User sent a message without ever typing /start
		msg := messages.NewBuilder(botUpdate.ChatID).
			WithText("Please type /start to begin\\.").
			Build()
		r.botClient.SendMessage(ctx, msg)
		return
	}

	// 6. Route callbacks
	if botUpdate.CallbackData != nil {
		for prefix, handler := range r.callbackHandlers {
			if strings.HasPrefix(*botUpdate.CallbackData, prefix) {
				ctxLogger.Info().Str("handler", prefix).Str("data", *botUpdate.CallbackData).Msg("Routing to callback handler")
				if err := handler.Handle(ctx, botUpdate, user); err != nil {
					ctxLogger.Error().Err(err).Msg("Callback handler failed")
				}
				return
			}
		}
		ctxLogger.Warn().Str("data", *botUpdate.CallbackData).Msg("No callback handler found")
		return
	}

	// 7. Route all other messages (Text, Contact, Photo)
	if r.messageHandler != nil {
		log := ctxLogger.With().Str("state", string(user.State)).Logger()
		if botUpdate.Contact != nil {
			log.Info().Msg("Routing contact message to text handler")
		} else if botUpdate.Photo != nil {
			log.Info().Msg("Routing photo message to text handler")
		} else {
			log.Info().Msg("Routing text message to text handler")
		}

		if err := r.messageHandler.Handle(ctx, botUpdate, user); err != nil {
			ctxLogger.Error().Err(err).Msg("Text handler failed")
		}
		return
	}

	// If we're here, it's an unhandled message
	ctxLogger.Info().Str("text", botUpdate.Text).Msg("Received unhandled message (no handler)")
}

// parseUpdate converts a tgbotapi.Update into our internal, simplified struct.
func (r *CustomerRouter) parseUpdate(update *tgbotapi.Update) (*ports.BotUpdate, bool) {
	if update.CallbackQuery != nil {
		// This is a Callback
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
		// This is a Message
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

	return nil, false // Unsupported update
}
