package handlers

import (
	"AsaExchange/internal/bot/customer"
	"AsaExchange/internal/bot/messages"
	"AsaExchange/internal/core/domain"
	"AsaExchange/internal/core/ports"
	"AsaExchange/internal/shared/config"
	"context"
	"fmt"

	"github.com/rs/zerolog"
)

// init registers this handler with the customer registry
func init() {
	customer.RegisterCallback(NewPolicyHandler)
}

type policyHandler struct {
	log      zerolog.Logger
	userRepo ports.UserRepository
	bot      ports.BotClientPort
}

// NewPolicyHandler creates a new handler for policy callbacks
func NewPolicyHandler(
	cfg *config.Config,
	userRepo ports.UserRepository,
	bot ports.BotClientPort,
	baseLogger *zerolog.Logger,
) ports.CallbackHandler {
	return &policyHandler{
		log:      baseLogger.With().Str("component", "policy_handler").Logger(),
		userRepo: userRepo,
		bot:      bot,
	}
}

// Prefix returns the prefix this handler is responsible for.
// We'll just use "policy_"
func (h *policyHandler) Prefix() string {
	return "policy_"
}

// Handle processes the "policy_accept" or "policy_decline" callbacks.
func (h *policyHandler) Handle(ctx context.Context, update *ports.BotUpdate, user *domain.User) error {
	log := h.log.With().Str("user_id", user.ID.String()).Logger()

	// 1. Answer the callback to stop the spinner
	h.bot.AnswerCallbackQuery(ctx, ports.AnswerCallbackParams{
		CallbackQueryID: update.CallbackQueryID,
	})

	// 2. Parse the callback data
	action := *update.CallbackData // This is "policy_accept" or "policy_decline"

	switch action {
	case "policy_accept":
		log.Info().Msg("User accepted policy. Completing registration.")

		// 1. Update user state
		user.State = domain.StateNone // Registration is complete
		// user.VerificationStatus is already 'pending'
		if err := h.userRepo.Update(ctx, user); err != nil {
			log.Error().Err(err).Msg("Failed to update user state after policy accept")
			return h.sendErrorMessage(ctx, update.ChatID, "An internal error occurred.")
		}

		// 2. Send confirmation and remove inline keyboard
		msg := messages.NewBuilder(update.ChatID).
			WithText(fmt.Sprintf(
				"✅ *Registration Complete\\!*\n\nThank you, %s\\. Your account is now submitted and *pending admin verification*\\.\n\nWe will notify you as soon as you are approved to make transactions\\.",
				*user.FirstName,
			)).
			Build()

		// 3. Edit the original policy message to remove the buttons
		h.bot.EditMessageText(ctx, ports.EditMessageParams{
			ChatID:    update.ChatID,
			MessageID: update.MessageID,
			Text:      "You have accepted the terms of service.",
		})

		_, err := h.bot.SendMessage(ctx, msg)
		return err
	case "policy_decline":
		log.Info().Msg("User declined policy. Resetting registration.")

		// 1. Reset user for re-registration
		user.State = domain.StateAwaitingFirstName
		user.FirstName = nil
		user.LastName = nil
		user.PhoneNumber = nil
		user.GovernmentID = nil
		user.IdentityDocRef = nil
		user.LocationCountry = nil
		user.VerificationStrategy = nil

		if err := h.userRepo.Update(ctx, user); err != nil {
			log.Error().Err(err).Msg("Failed to reset user state after policy decline")
			return h.sendErrorMessage(ctx, update.ChatID, "An internal error occurred.")
		}

		// 2. Send message
		msg := messages.NewBuilder(update.ChatID).
			WithText("You have declined the terms\\. To use this bot, you must accept the terms\\.\n\nThe registration process will now restart\\. Please reply with your *legal First Name*\\.").
			Build()

		// 3. Edit the original policy message
		h.bot.EditMessageText(ctx, ports.EditMessageParams{
			ChatID:    update.ChatID,
			MessageID: update.MessageID,
			Text:      "You have declined the terms of service.",
		})

		_, err := h.bot.SendMessage(ctx, msg)
		return err
	}
	return nil
}

// sendErrorMessage is a helper to send a generic error
func (h *policyHandler) sendErrorMessage(ctx context.Context, chatID int64, message string) error {
	msgParams := messages.NewBuilder(chatID).
		WithText(message).
		WithParseMode("").Build()
	_, err := h.bot.SendMessage(ctx, msgParams)
	return err
}
