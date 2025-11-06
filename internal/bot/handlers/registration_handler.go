package handlers

import (
	"AsaExchange/internal/core/domain"
	"AsaExchange/internal/core/ports"
	"context"

	"github.com/rs/zerolog"
)

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
) ports.StateHandler {
	return &registrationHandler{
		log:      baseLogger.With().Str("component", "reg_handler").Logger(),
		userRepo: userRepo,
		bot:      bot,
	}
}

// State
func (h *registrationHandler) State() domain.UserState {
	return domain.StateAwaitingFirstName
}

// Handle
func (h *registrationHandler) Handle(ctx context.Context, update *ports.BotUpdate, user *domain.User) error {
	log := h.log.With().Str("user_id", user.ID.String()).Logger()

	firstName := update.Text

	// Basic validation (we can make this stronger later)
	if len(firstName) < 2 || len(firstName) > 50 {
		msgParams := ports.SendMessageParams{
			ChatID:    update.ChatID,
			Text:      "Invalid first name\\. Please enter a name between 2 and 50 characters\\.",
			ParseMode: "MarkdownV2",
		}
		return h.bot.SendMessage(ctx, msgParams)
	}

	// 1. Modify the user struct in-place
	user.FirstName = &firstName
	user.State = domain.StateAwaitingLastName

	// 2. Call the generic Update method
	log.Info().Str("first_name", firstName).Msg("Updating user's first name and state")
	if err := h.userRepo.Update(ctx, user); err != nil {
		log.Error().Err(err).Msg("Failed to update user")
		return err
	}

	// 3. Ask for the next piece of information
	msgParams := ports.SendMessageParams{
		ChatID:    update.ChatID,
		Text:      "Thank you\\. Now, please reply with your **legal Last Name**\\.",
		ParseMode: "MarkdownV2",
	}

	return h.bot.SendMessage(ctx, msgParams)
}
