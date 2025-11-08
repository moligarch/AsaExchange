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
	log := h.log.With().Int64("user_id", update.UserID).Logger()
	ctx = log.WithContext(ctx)

	user, err := h.userRepo.GetByTelegramID(ctx, update.UserID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get user from repository")
		return h.sendErrorMessage(ctx, update.ChatID)
	}

	var msg ports.SendMessageParams

	if user == nil {
		// --- CASE 1: NEW USER ---
		log.Info().Msg("New user found. Creating account and prompting for registration.")

		newUser := &domain.User{
			ID:                 uuid.New(),
			TelegramID:         update.UserID,
			VerificationStatus: domain.VerificationPending,
			State:              domain.StateAwaitingFirstName,
		}

		if err := h.userRepo.Create(ctx, newUser); err != nil {
			log.Error().Err(err).Msg("Failed to create new user")
			return h.sendErrorMessage(ctx, update.ChatID)
		}
		log.Info().Str("user_id", newUser.ID.String()).Msg("New user created successfully")

		text := "👋 Welcome to AsaExchange\\!\n\nTo use our service, you must first register an account\\.\n\n"
		text += "Please reply with your *legal First Name* as it appears on your ID\\."
		msg = messages.NewBuilder(update.ChatID).WithText(text).WithRemoveKeyboard().Build()

	} else {
		// --- CASE 2: EXISTING USER ---
		log.Info().Str("user_id", user.ID.String()).Str("status", string(user.VerificationStatus)).Msg("Existing user found.")

		var responseText string
		switch user.VerificationStatus {
		case domain.VerificationPending:
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
				_, err := h.bot.SendMessage(ctx, msg)
				return err
			case domain.StateAwaitingGovID:
				responseText = "Please reply with your *Government ID / National ID Number*\\."
			case domain.StateAwaitingLocation:
				var countryButtons []string
				for _, conf := range h.countryStrategies {
					countryButtons = append(countryButtons, conf.Title)
				}
				msg = messages.NewBuilder(update.ChatID).
					WithText("Please select your *Country of Residence* from the list\\.").
					WithReplyButtons(countryButtons, 2).
					Build()
				_, err := h.bot.SendMessage(ctx, msg)
				return err
			case domain.StateAwaitingIdentityDoc:
				responseText = "Please upload a *single, clear photo* of your Government ID or Passport\\."
			case domain.StateAwaitingPolicyApproval:
				responseText = "Please review our terms of service and *accept or decline* the policy\\."
			case domain.StateNone:
				if user.FirstName != nil {
					responseText = fmt.Sprintf(
						"Hello, %s\\. Your account is still *pending verification*\\. Please wait for an admin to approve your identity\\.",
						*user.FirstName,
					)
				} else {
					responseText = "Your account is still *pending verification*\\. Please wait\\."
				}
			default:
				responseText = "Your account is still *pending verification*\\. Please wait\\."
			}

		case domain.VerificationRejected:
			log.Info().Msg("User is 'rejected'. Resetting state for re-registration.")
			user.State = domain.StateAwaitingFirstName
			user.VerificationStatus = domain.VerificationPending
			user.FirstName = nil
			user.LastName = nil
			user.PhoneNumber = nil
			user.GovernmentID = nil
			user.IdentityDocRef = nil
			user.LocationCountry = nil

			if err := h.userRepo.Update(ctx, user); err != nil {
				log.Error().Err(err).Msg("Failed to reset user state for re-registration")
				return h.sendErrorMessage(ctx, update.ChatID)
			}

			responseText = "Your previous registration was rejected\\.\n\nYou may try again\\. Please reply with your *legal First Name*\\."

		case domain.VerificationLevel1:
			responseText = fmt.Sprintf(
				"👋 Welcome back, %s\\! Use the menu to get started\\.",
				*user.FirstName,
			)
		}

		if msg.Text == "" {
			msg = messages.NewBuilder(update.ChatID).WithText(responseText).WithRemoveKeyboard().Build()
		}
	}

	_, err = h.bot.SendMessage(ctx, msg)
	return err
}

// sendErrorMessage is a helper to send a generic error
func (h *startHandler) sendErrorMessage(ctx context.Context, chatID int64) error {
	msgParams := messages.NewBuilder(chatID).
		WithText("An internal error occurred. Please try again later.").
		WithParseMode("").Build() // Use plain text for simple error
	_, err := h.bot.SendMessage(ctx, msgParams)
	return err
}
