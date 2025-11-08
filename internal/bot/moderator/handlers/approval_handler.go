package handlers

import (
	"AsaExchange/internal/bot/moderator"
	"AsaExchange/internal/core/domain"
	"AsaExchange/internal/core/ports"
	"AsaExchange/internal/shared/config"
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// init
func init() {
	moderator.RegisterCallback(NewApprovalHandler)
}

// approvalHandler
type approvalHandler struct {
	log      zerolog.Logger
	userRepo ports.UserRepository
	bot      ports.BotClientPort
	bus      ports.EventBus
}

// NewApprovalHandler
func NewApprovalHandler(
	cfg *config.Config,
	userRepo ports.UserRepository,
	bot ports.BotClientPort,
	bus ports.EventBus,
	baseLogger *zerolog.Logger,
) ports.CallbackHandler {
	return &approvalHandler{
		log:      baseLogger.With().Str("component", "approval_handler").Logger(),
		userRepo: userRepo,
		bot:      bot,
		bus:      bus,
	}
}

func (h *approvalHandler) Prefix() string {
	return "approval_"
}

func (h *approvalHandler) Handle(ctx context.Context, update *ports.BotUpdate, adminUser *domain.User) error {
	log := h.log.With().Int64("admin_id", adminUser.TelegramID).Logger()

	// 1. Answer the callback to stop the spinner
	h.bot.AnswerCallbackQuery(ctx, ports.AnswerCallbackParams{
		CallbackQueryID: update.CallbackQueryID,
	})

	// 2. Parse the callback data
	parts := strings.Split(*update.CallbackData, "_")
	if len(parts) != 3 {
		log.Error().Str("data", *update.CallbackData).Msg("Invalid callback data format")
		return nil
	}

	action := parts[1] // "accept" or "reject"
	userID, err := uuid.Parse(parts[2])
	if err != nil {
		log.Error().Err(err).Str("user_id_str", parts[2]).Msg("Failed to parse UUID from callback")
		return nil
	}

	log = log.With().Str("target_user_id", userID.String()).Str("action", action).Logger()

	// 3. Get the user to be approved/rejected
	user, err := h.userRepo.GetByID(ctx, userID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get target user by ID")
		return h.editMessage(ctx, update, "Error: Could not find user.")
	}
	if user == nil {
		log.Error().Msg("Target user not found, though GetByID returned no error")
		return h.editMessage(ctx, update, "Error: Could not find user.")
	}

	// 4. Process the action
	switch action {
	case "accept":
		user.VerificationStatus = domain.VerificationLevel1
		user.State = domain.StateNone // Registration complete

		if err := h.userRepo.Update(ctx, user); err != nil {
			log.Error().Err(err).Msg("Failed to update user to 'level_1'")
			return h.editMessage(ctx, update, "Error: Could not update user.")
		}

		log.Info().Msg("User approved")

		// Publish an event instead of sending a message
		if err := h.bus.Publish(ctx, "user:approved", user); err != nil {
			log.Error().Err(err).Msg("Failed to publish 'user:approved' event")
			// Don't fail the whole operation, just log the error
		}

		// Edit the admin's message
		return h.editMessage(ctx, update, fmt.Sprintf("✅ User Approved: %s %s", *user.FirstName, *user.LastName))

	case "reject":
		// As per your request: reset them for re-registration
		user.VerificationStatus = domain.VerificationRejected
		user.State = domain.StateAwaitingFirstName
		user.FirstName = nil
		user.LastName = nil
		user.PhoneNumber = nil
		user.GovernmentID = nil
		user.IdentityDocRef = nil
		user.LocationCountry = nil
		user.VerificationStrategy = nil

		if err := h.userRepo.Update(ctx, user); err != nil {
			log.Error().Err(err).Msg("Failed to update user to 'rejected'")
			return h.editMessage(ctx, update, "Error: Could not update user.")
		}

		log.Info().Msg("User rejected")

		// Publish an event instead of sending a message
		if err := h.bus.Publish(ctx, "user:rejected", user); err != nil {
			log.Error().Err(err).Msg("Failed to publish 'user:rejected' event")
		}

		// Edit the admin's message
		return h.editMessage(ctx, update, "❌ User Rejected")
	}

	return nil
}

// editMessage (UNCHANGED)
func (h *approvalHandler) editMessage(ctx context.Context, update *ports.BotUpdate, text string) error {
	msg := ports.EditMessageCaptionParams{
		ChatID:      update.ChatID,
		MessageID:   update.MessageID,
		Caption:     text,
		ParseMode:   "",  // Plain text
		ReplyMarkup: nil, // Remove buttons
	}
	return h.bot.EditMessageCaption(ctx, msg)
}
