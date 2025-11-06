package handlers

import (
	"AsaExchange/internal/core/domain"
	"AsaExchange/internal/core/ports"
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// startHandler is the plugin for the /start command.
type startHandler struct {
	log      zerolog.Logger
	userRepo ports.UserRepository
	bot      ports.BotClientPort
}

// NewStartHandler creates a new handler for the /start command.
func NewStartHandler(
	userRepo ports.UserRepository,
	bot ports.BotClientPort,
	baseLogger *zerolog.Logger,
) ports.CommandHandler {
	return &startHandler{
		log:      baseLogger.With().Str("component", "start_handler").Logger(),
		userRepo: userRepo,
		bot:      bot,
	}
}

// Command returns the command string (without the "/")
func (h *startHandler) Command() string {
	return "start"
}

// Handle processes the /start command with the new logic.
func (h *startHandler) Handle(ctx context.Context, update *ports.BotUpdate) error {
	ctxLogger := h.log.With().Int64("user_id", update.UserID).Logger()
	ctx = ctxLogger.WithContext(ctx)

	// 1. Check if user exists
	user, err := h.userRepo.GetByTelegramID(ctx, update.UserID)
	if err != nil {
		ctxLogger.Error().Err(err).Msg("Failed to get user from repository")
		h.sendErrorMessage(ctx, update.ChatID)
		return err
	}

	// 2. Handle the user based on their status
	var responseText string

	if user == nil {
		// --- CASE 1: NEW USER ---
		ctxLogger.Info().Msg("New user found. Creating account and prompting for registration.")

		newUser := &domain.User{
			ID:                 uuid.New(),
			TelegramID:         update.UserID,
			FirstName:          nil,
			LastName:           nil,
			VerificationStatus: domain.VerificationPending,
			State:              domain.StateAwaitingFirstName,
			IsModerator:        false,
		}

		if err := h.userRepo.Create(ctx, newUser); err != nil {
			ctxLogger.Error().Err(err).Msg("Failed to create new user")
			h.sendErrorMessage(ctx, update.ChatID)
			return err
		}

		ctxLogger.Info().Str("user_id", newUser.ID.String()).Msg("New user created successfully")

		responseText = "👋 Welcome to AsaExchange\\!\n\nTo use our service, you must first register an account\\.\n\n"
		responseText += "Please reply with your **legal First Name** as it appears on your ID\\."

	} else {
		// --- CASE 2: EXISTING USER ---
		ctxLogger.Info().Str("user_id", user.ID.String()).Str("status", string(user.VerificationStatus)).Msg("Existing user found.")

		switch user.VerificationStatus {
		case domain.VerificationPending:
			// Check their state to see if they are in the middle of registration
			switch user.State {
			case domain.StateAwaitingFirstName:
				responseText = "Please reply with your **legal First Name** as it appears on your ID\\."
			case domain.StateAwaitingLastName:
				responseText = "Please reply with your **legal Last Name** as it appears on your ID\\."
			// Add other states (phone, gov_id) here later
			case domain.StateAwaitingPhoneNumber, domain.StateAwaitingGovID, domain.StateAwaitingLocation, domain.StateAwaitingPolicyApproval:
				responseText = "Your registration is in progress\\. Please follow the instructions\\."
			case domain.StateNone:
				responseText = fmt.Sprintf(
					"Hello, %s. Your account is still **pending verification**\\. Please wait for an admin to approve your identity\\.",
					*user.FirstName, // Assumes first name is set by now
				)
			default:
				responseText = "Your account is still **pending verification**\\. Please wait\\."
			}

		case domain.VerificationRejected:
			responseText = "There was an issue with your identity verification\\. Please contact support\\."
		case domain.VerificationLevel1:
			responseText = fmt.Sprintf(
				"👋 Welcome back, %s\\! Use the menu to get started\\.",
				*user.FirstName,
			)
		}
	}

	// 3. Send the appropriate message
	msgParams := ports.SendMessageParams{
		ChatID:    update.ChatID,
		Text:      responseText,
		ParseMode: "MarkdownV2",
	}

	if err := h.bot.SendMessage(ctx, msgParams); err != nil {
		ctxLogger.Error().Err(err).Msg("Failed to send message")
		return err
	}

	return nil
}

// sendErrorMessage is a helper to send a generic error
func (h *startHandler) sendErrorMessage(ctx context.Context, chatID int64) {
	msgParams := ports.SendMessageParams{
		ChatID: chatID,
		Text:   "An internal error occurred. Please try again later.",
	}
	h.bot.SendMessage(ctx, msgParams)
}
