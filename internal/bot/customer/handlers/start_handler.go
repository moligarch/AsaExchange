package handlers

import (
	"AsaExchange/internal/bot/customer"
	"AsaExchange/internal/bot/messages"
	"AsaExchange/internal/core/domain"
	"AsaExchange/internal/core/ports"
	"AsaExchange/internal/shared/config"
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

func init() {
	customer.RegisterCommand(NewStartHandler)
}

// startHandler is the plugin for the /start command.
type startHandler struct {
	log               zerolog.Logger
	userRepo          ports.UserRepository
	bot               ports.BotClientPort
	countryStrategies map[string]config.CountryConfig
}

// NewStartHandler creates a new handler for the /start command.
func NewStartHandler(
	cfg *config.Config,
	userRepo ports.UserRepository,
	bot ports.BotClientPort,
	baseLogger *zerolog.Logger,
) ports.CommandHandler {
	return &startHandler{
		log:               baseLogger.With().Str("component", "start_handler").Logger(),
		userRepo:          userRepo,
		bot:               bot,
		countryStrategies: cfg.Bot.Customer.CountryStrategies,
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
		return h.sendErrorMessage(ctx, update.ChatID)
	}

	// 2. Handle the user based on their status
	var msg ports.SendMessageParams

	if user == nil {
		// --- CASE 1: NEW USER ---
		ctxLogger.Info().Msg("New user found. Creating account and prompting for registration.")

		newUser := &domain.User{
			ID:                 uuid.New(),
			TelegramID:         update.UserID,
			VerificationStatus: domain.VerificationPending,
			State:              domain.StateAwaitingFirstName,
		}

		if err := h.userRepo.Create(ctx, newUser); err != nil {
			ctxLogger.Error().Err(err).Msg("Failed to create new user")
			return h.sendErrorMessage(ctx, update.ChatID)
		}

		ctxLogger.Info().Str("user_id", newUser.ID.String()).Msg("New user created successfully")

		text := "👋 Welcome to AsaExchange\\!\n\nTo use our service, you must first register an account\\.\n\n"
		text += "Please reply with your *legal First Name* as it appears on your ID\\."
		msg = messages.NewBuilder(update.ChatID).WithText(text).WithRemoveKeyboard().Build()

	} else {
		// --- CASE 2: EXISTING USER ---
		ctxLogger.Info().Str("user_id", user.ID.String()).Str("status", string(user.VerificationStatus)).Msg("Existing user found.")

		var responseText string
		switch user.VerificationStatus {
		case domain.VerificationPending:
			// Check their state to see if they are in the middle of registration
			switch user.State {
			case domain.StateAwaitingFirstName:
				responseText = "Please reply with your *legal First Name* as it appears on your ID\\."
			case domain.StateAwaitingLastName:
				responseText = "Please reply with your *legal Last Name* as it appears on your ID\\."
			case domain.StateAwaitingPhoneNumber:
				msg = messages.NewBuilder(update.ChatID).
					WithText("Please share your *Phone Number* by pressing the button below\\.").
					WithContactButton("Share My Phone Number").
					Build()
			case domain.StateAwaitingGovID:
				responseText = "Please reply with your *Government ID / National ID Number*\\."
			case domain.StateAwaitingLocation:
				var countryButtons []string
				for title := range h.countryStrategies {
					countryButtons = append(countryButtons, title)
				}
				msg = messages.NewBuilder(update.ChatID).
					WithText("Please select your *Country of Residence* from the list\\.").
					WithReplyButtons(countryButtons, 2).
					Build()
				return h.bot.SendMessage(ctx, msg)
			case domain.StateAwaitingIdentityDoc:
				responseText = "Please upload a *single, clear photo* of your Government ID or Passport\\."
			case domain.StateAwaitingPolicyApproval:
				responseText = "Please review our terms of service and *accept or decline* the policy\\."
			default:
				if user.FirstName != nil {
					responseText = fmt.Sprintf(
						"Hello, %s\\. Your account is still *pending verification*\\. Please wait for an admin to approve your identity\\.",
						*user.FirstName,
					)
				} else {
					responseText = "Your account is still *pending verification*\\. Please wait\\."
				}
			}

		case domain.VerificationRejected:
			ctxLogger.Info().Msg("User is 'rejected'. Resetting state for re-registration.")
			user.State = domain.StateAwaitingFirstName
			user.VerificationStatus = domain.VerificationPending
			user.FirstName = nil
			user.LastName = nil
			user.PhoneNumber = nil
			user.GovernmentID = nil
			user.GovernmentIDPhotoID = nil
			user.LocationCountry = nil
			
			if err := h.userRepo.Update(ctx, user); err != nil {
				ctxLogger.Error().Err(err).Msg("Failed to reset user state for re-registration")
				return h.sendErrorMessage(ctx, update.ChatID)
			}
			
			responseText = "Your previous registration was rejected\\.\n\nYou may try again\\. Please reply with your *legal First Name*\\."
		
		case domain.VerificationLevel1:
			responseText = fmt.Sprintf(
				"👋 Welcome back, %s\\! Use the menu to get started\\.",
				*user.FirstName,
			)
			msg = messages.NewBuilder(update.ChatID).WithText(responseText).WithRemoveKeyboard().Build()
		}

		// If we didn't already build a special message, build a simple text one.
		if msg.Text == "" {
			msg = messages.NewBuilder(update.ChatID).WithText(responseText).Build()
		}
	}

	return h.bot.SendMessage(ctx, msg)
}

// sendErrorMessage is a helper to send a generic error
func (h *startHandler) sendErrorMessage(ctx context.Context, chatID int64) error {
	msgParams := messages.NewBuilder(chatID).
		WithText("An internal error occurred. Please try again later.").
		WithParseMode("").Build() // Use plain text for simple error
	return h.bot.SendMessage(ctx, msgParams)
}
