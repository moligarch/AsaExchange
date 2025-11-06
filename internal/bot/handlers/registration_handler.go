package handlers

import (
	"AsaExchange/internal/bot"
	"AsaExchange/internal/core/domain"
	"AsaExchange/internal/core/ports"
	"context"

	"github.com/rs/zerolog"
)

func init() {
	bot.RegisterText(NewRegistrationHandler)
}

// registrationHandler
type registrationHandler struct {
	log      zerolog.Logger
	userRepo ports.UserRepository
	bot      ports.BotClientPort
}

// NewRegistrationHandler
func NewRegistrationHandler(
	userRepo ports.UserRepository,
	bot ports.BotClientPort,
	baseLogger *zerolog.Logger,
) ports.TextHandler {
	return &registrationHandler{
		log:      baseLogger.With().Str("component", "reg_handler").Logger(),
		userRepo: userRepo,
		bot:      bot,
	}
}

// Handle is the main entry point for all text replies.
// It routes logic based on the user's state.
func (h *registrationHandler) Handle(ctx context.Context, update *ports.BotUpdate, user *domain.User) error {

	// --- THE STATE MACHINE ---
	switch user.State {
	case domain.StateAwaitingFirstName:
		return h.handleFirstName(ctx, update, user)
	case domain.StateAwaitingLastName:
		return h.handleLastName(ctx, update, user)

	// Add new states here as we build them
	// case domain.StateAwaitingPhoneNumber:
	// 	 return h.handlePhoneNumber(ctx, update, user)

	default:
		// User is in a state we don't handle (e.g., "none")
		h.log.Warn().Str("state", string(user.State)).Msg("Received text in unhandled state")
		// Optionally send a "I don't understand" message
		return nil
	}
}

// handleFirstName processes the user's first name submission.
func (h *registrationHandler) handleFirstName(ctx context.Context, update *ports.BotUpdate, user *domain.User) error {
	log := h.log.With().Str("user_id", user.ID.String()).Logger()

	firstName := update.Text

	// Basic validation
	if len(firstName) < 2 || len(firstName) > 50 {
		msgParams := ports.SendMessageParams{
			ChatID:    update.ChatID,
			Text:      "Invalid first name\\. Please enter a name between 2 and 50 characters\\.",
			ParseMode: "MarkdownV2",
		}
		return h.bot.SendMessage(ctx, msgParams)
	}

	// 1. Modify the user struct
	user.FirstName = &firstName
	user.State = domain.StateAwaitingLastName // Move to the next state

	// 2. Call the generic Update method
	log.Info().Str("first_name", firstName).Msg("Updating user's first name and state")
	if err := h.userRepo.Update(ctx, user); err != nil {
		log.Error().Err(err).Msg("Failed to update user")
		return h.sendErrorMessage(ctx, update.ChatID)
	}

	// 3. Ask for the next piece of information
	msgParams := ports.SendMessageParams{
		ChatID:    update.ChatID,
		Text:      "Thank you\\. Now, please reply with your **legal Last Name**\\.",
		ParseMode: "MarkdownV2",
	}

	return h.bot.SendMessage(ctx, msgParams)
}

// handleLastName processes the user's last name submission.
func (h *registrationHandler) handleLastName(ctx context.Context, update *ports.BotUpdate, user *domain.User) error {
	log := h.log.With().Str("user_id", user.ID.String()).Logger()

	lastName := update.Text

	// Basic validation
	if len(lastName) < 2 || len(lastName) > 50 {
		msgParams := ports.SendMessageParams{
			ChatID:    update.ChatID,
			Text:      "Invalid last name\\. Please enter a name between 2 and 50 characters\\.",
			ParseMode: "MarkdownV2",
		}
		return h.bot.SendMessage(ctx, msgParams)
	}

	// 1. Modify the user struct
	user.LastName = &lastName
	user.State = domain.StateAwaitingPhoneNumber // Move to the next state

	// 2. Call the generic Update method
	log.Info().Str("last_name", lastName).Msg("Updating user's last name and state")
	if err := h.userRepo.Update(ctx, user); err != nil {
		log.Error().Err(err).Msg("Failed to update user")
		return h.sendErrorMessage(ctx, update.ChatID)
	}

	// 3. Ask for the next piece of information
	msgParams := ports.SendMessageParams{
		ChatID:    update.ChatID,
		Text:      "Thank you\\. Now, please reply with your **Phone Number** (including country code, e\\.g\\., \\+98912\\.\\.\\. or \\+49151\\.\\.\\.)\\.",
		ParseMode: "MarkdownV2",
	}

	return h.bot.SendMessage(ctx, msgParams)
}

// sendErrorMessage is a helper to send a generic error
func (h *registrationHandler) sendErrorMessage(ctx context.Context, chatID int64) error {
	msgParams := ports.SendMessageParams{
		ChatID: chatID,
		Text:   "An internal error occurred. Please try again later.",
	}
	// We use a background context for this because the original
	// request context might be cancelled.
	return h.bot.SendMessage(ctx, msgParams)
}
