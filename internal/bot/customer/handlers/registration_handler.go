package handlers

import (
	"AsaExchange/internal/bot/customer"
	"AsaExchange/internal/bot/messages"
	"AsaExchange/internal/core/domain"
	"AsaExchange/internal/core/ports"
	"AsaExchange/internal/shared/config"
	"context"
	"fmt"
	"regexp"

	"github.com/rs/zerolog"
)

func init() {
	customer.RegisterMessage(NewRegistrationHandler)
}

var phoneRegex = regexp.MustCompile(`^\+?[0-9]{9,15}$`)

// registrationHandler
type registrationHandler struct {
	log               zerolog.Logger
	userRepo          ports.UserRepository
	bot               ports.BotClientPort
	countryStrategies map[string]config.CountryConfig
}

// NewRegistrationHandler
func NewRegistrationHandler(
	cfg *config.Config,
	userRepo ports.UserRepository,
	bot ports.BotClientPort,
	baseLogger *zerolog.Logger,
) ports.MessageHandler {
	return &registrationHandler{
		log:               baseLogger.With().Str("component", "reg_handler").Logger(),
		userRepo:          userRepo,
		bot:               bot,
		countryStrategies: cfg.Bot.Customer.CountryStrategies,
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
	case domain.StateAwaitingPhoneNumber:
		return h.handlePhoneNumber(ctx, update, user)
	case domain.StateAwaitingGovID:
		return h.handleGovID(ctx, update, user)
	case domain.StateAwaitingLocation:
		return h.handleLocation(ctx, update, user)
	case domain.StateAwaitingIdentityDoc:
		return h.handleIdentityDoc(ctx, update, user)
	case domain.StateAwaitingPolicyApproval:
		return h.handlePolicyApproval(ctx, update, user)

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

	if update.Contact != nil {
		msg := messages.NewBuilder(update.ChatID).
		WithText("Please reply with your First Name as text.").
		WithParseMode("").Build()
		return h.bot.SendMessage(ctx, msg)
	}

	firstName := update.Text

	// Basic validation
	if len(firstName) < 2 || len(firstName) > 50 {
		msg := messages.NewBuilder(update.ChatID).
			WithText("Invalid first name. Please enter a name between 2 and 50 characters.").
			WithParseMode("").Build()
		return h.bot.SendMessage(ctx, msg)
	}

	// 1. Modify the user struct
	user.FirstName = &firstName
	user.State = domain.StateAwaitingLastName // Move to the next state

	// 2. Call the generic Update method
	log.Info().Str("first_name", firstName).Msg("Updating user's first name and state")
	if err := h.userRepo.Update(ctx, user); err != nil {
		log.Error().Err(err).Msg("Failed to update user")
		return h.sendErrorMessage(ctx, update.ChatID, "An internal error occurred.")
	}

	// 3. Ask for the next piece of information
	msg := messages.NewBuilder(update.ChatID).
		WithText("Thank you\\. Now, please reply with your *legal Last Name*\\.").
		Build()

	return h.bot.SendMessage(ctx, msg)
}

// handleLastName processes the user's last name submission.
func (h *registrationHandler) handleLastName(ctx context.Context, update *ports.BotUpdate, user *domain.User) error {
	log := h.log.With().Str("user_id", user.ID.String()).Logger()

	if update.Contact != nil {
		msg := messages.NewBuilder(update.ChatID).
			WithText("Please reply with your Last Name as text.").
			WithParseMode("").
			Build()
		return h.bot.SendMessage(ctx, msg)
	}

	lastName := update.Text

	// Basic validation
	if len(lastName) < 2 || len(lastName) > 50 {
		msg := messages.NewBuilder(update.ChatID).
			WithText("Invalid last name. Please enter a name between 2 and 50 characters.").
			WithParseMode("").
			Build()
		return h.bot.SendMessage(ctx, msg)
	}

	// 1. Modify the user struct
	user.LastName = &lastName
	user.State = domain.StateAwaitingPhoneNumber // Move to the next state

	// 2. Call the generic Update method
	log.Info().Str("last_name", lastName).Msg("Updating user's last name and state")
	if err := h.userRepo.Update(ctx, user); err != nil {
		log.Error().Err(err).Msg("Failed to update user")
		return h.sendErrorMessage(ctx, update.ChatID, "An internal error occurred.")
	}

	// 3. Ask for the next piece of information
	msg := messages.NewBuilder(update.ChatID).
		WithText("Thank you\\. Now, please share your *Phone Number* by pressing the button below\\.").
		WithContactButton("Share My Phone Number").
		Build()

	return h.bot.SendMessage(ctx, msg)
}

func (h *registrationHandler) handlePhoneNumber(ctx context.Context, update *ports.BotUpdate, user *domain.User) error {
	log := h.log.With().Str("user_id", user.ID.String()).Logger()

	if update.Contact == nil {
		msg := messages.NewBuilder(update.ChatID).
			WithText("Please press the *Share My Phone Number* button to continue\\.").
			WithContactButton("Share My Phone Number"). // Re-send the button
			Build()
		return h.bot.SendMessage(ctx, msg)
	}

	if update.Contact.UserID != update.UserID {
		log.Warn().Int64("contact_id", update.Contact.UserID).Msg("User tried to share someone else's contact")
		msg := messages.NewBuilder(update.ChatID).
			WithText("You must share your *own* contact\\. Please press the button again\\.").
			WithContactButton("Share My Phone Number"). // Re-send the button
			Build()
		return h.bot.SendMessage(ctx, msg)
	}

	phoneNumber := update.Contact.PhoneNumber
	if !phoneRegex.MatchString(phoneNumber) {
		log.Error().Str("phone", phoneNumber).Msg("User shared an invalid phone number format")
		msg := messages.NewBuilder(update.ChatID).
			WithText("The phone number you shared has an invalid format. Please contact support.").
			WithRemoveKeyboard().
			WithParseMode("").
			Build()
		return h.bot.SendMessage(ctx, msg)
	}

	user.PhoneNumber = &phoneNumber
	user.State = domain.StateAwaitingGovID

	log.Info().Str("phone", phoneNumber).Msg("Updating user's phone number and state")
	if err := h.userRepo.Update(ctx, user); err != nil {
		log.Error().Err(err).Msg("Failed to update user")
		return h.sendErrorMessage(ctx, update.ChatID, "An internal error occurred.")
	}

	// Use the builder to remove the keyboard and ask the next question
	msg := messages.NewBuilder(update.ChatID).
		WithText("Thank you\\. Finally, please reply with your *Government ID / National ID Number*\\.").
		WithRemoveKeyboard().
		Build()

	return h.bot.SendMessage(ctx, msg)
}

// handleGovID processes the user's government ID submission.
func (h *registrationHandler) handleGovID(ctx context.Context, update *ports.BotUpdate, user *domain.User) error {
	log := h.log.With().Str("user_id", user.ID.String()).Logger()

	if update.Contact != nil || update.Photo != nil {
		return h.sendErrorMessage(ctx, update.ChatID, "Please reply with your Government ID as text.")
	}

	govID := update.Text

	// Basic validation (e.g., must be at least 5 chars)
	if len(govID) < 5 || len(govID) > 50 {
		msg := messages.NewBuilder(update.ChatID).
			WithText("Invalid ID format\\. Please reply with your *Government ID / National ID Number*\\.").
			Build()
		return h.bot.SendMessage(ctx, msg)
	}

	// 1. Modify the user struct
	user.GovernmentID = &govID
	user.State = domain.StateAwaitingLocation // Move to the next state

	// 2. Call the generic Update method
	log.Info().Msg("Updating user's government ID and state")
	if err := h.userRepo.Update(ctx, user); err != nil {
		log.Error().Err(err).Msg("Failed to update user")
		return h.sendErrorMessage(ctx, update.ChatID, "An internal error occurred.")
	}

	// 3. Ask for the next piece of information
	// Use the config to build buttons
	var countryButtons []string
	for _, conf := range h.countryStrategies {
		countryButtons = append(countryButtons, conf.Title)
	}
	msg := messages.NewBuilder(update.ChatID).
		WithText(fmt.Sprintf(
			"Thank you, %s\\.\n\nYour registration is almost complete\\. Please select your *Country of Residence* from the list below\\.",
			*user.FirstName,
		)).
		WithReplyButtons(countryButtons, 2). // Build a 2-column grid
		Build()

	return h.bot.SendMessage(ctx, msg)
}

// handleLocation processes the user's country selection.
func (h *registrationHandler) handleLocation(ctx context.Context, update *ports.BotUpdate, user *domain.User) error {
	log := h.log.With().Str("user_id", user.ID.String()).Logger()

	countryChoice := update.Text

	// 1. Validate the choice
	var isoKey string
	var countryConfig config.CountryConfig
	var ok bool

	// Find the key by the value
	for key, conf := range h.countryStrategies {
		if conf.Title == countryChoice {
			isoKey = key
			countryConfig = conf
			ok = true
			break
		}
	}

	if !ok {
		// User typed something, or a country we don't support.
		log.Warn().Str("choice", countryChoice).Msg("User selected an unsupported country")

		var countryButtons []string
		for _, conf := range h.countryStrategies {
			countryButtons = append(countryButtons, conf.Title)
		}

		msg := messages.NewBuilder(update.ChatID).
			WithText(fmt.Sprintf(
				"`%s` is not a supported country\\. Please select one from the list\\.",
				countryChoice,
			)).
			WithReplyButtons(countryButtons, 2).
			Build()
		return h.bot.SendMessage(ctx, msg)
	}

	// 2. Update the user
	user.LocationCountry = &isoKey
	user.VerificationStrategy = &countryConfig.Strategy
	user.State = domain.StateAwaitingIdentityDoc

	log.Info().Str("country", isoKey).Str("strategy", countryConfig.Strategy).Msg("Updating user's location and strategy")
	if err := h.userRepo.Update(ctx, user); err != nil {
		log.Error().Err(err).Msg("Failed to update user")
		return h.sendErrorMessage(ctx, update.ChatID, "An internal error occurred.")
	}

	// 3. Send next step (ask for photo)
	msg := messages.NewBuilder(update.ChatID).
		WithText(
			"Thank you\\. As the next step, please upload a *single, clear photo* of your Government ID or Passport\\.\n\nThis photo will be reviewed by an admin to verify your identity\\.",
		).
		WithRemoveKeyboard(). // Remove the country buttons
		Build()

	return h.bot.SendMessage(ctx, msg)
}

func (h *registrationHandler) handleIdentityDoc(ctx context.Context, update *ports.BotUpdate, user *domain.User) error {
	log := h.log.With().Str("user_id", user.ID.String()).Logger()

	// 1. Validate that it's a photo
	if update.Photo == nil {
		msg := messages.NewBuilder(update.ChatID).
			WithText("Please upload a *photo* of your ID, not text\\.").
			Build()
		return h.bot.SendMessage(ctx, msg)
	}

	// 2. Get the FileID
	fileID := update.Photo.FileID

	// 3. Update the user
	user.GovernmentIDPhotoID = &fileID
	user.State = domain.StateAwaitingPolicyApproval // <-- Set to next state

	log.Info().Str("file_id", fileID).Msg("Updating user's photo ID and moving to policy approval")
	if err := h.userRepo.Update(ctx, user); err != nil {
		log.Error().Err(err).Msg("Failed to update user")
		return h.sendErrorMessage(ctx, update.ChatID, "An internal error occurred.")
	}

	// 4. Send the policy message with inline buttons
	policyText := "Please review our terms of service and privacy policy\\.\n\n[Link to Policy](https://example.com/terms)\n\nDo you accept these terms\\?"

	msg := messages.NewBuilder(update.ChatID).
		WithText(policyText).
		WithInlineButtons([][]ports.Button{
			{
				{Text: "✅ I Accept", Data: "policy_accept"},
				{Text: "❌ I Decline", Data: "policy_decline"},
			},
		}).
		Build()

	return h.bot.SendMessage(ctx, msg)
}

// handlePolicyApproval handles text replies when user should be pressing buttons.
func (h *registrationHandler) handlePolicyApproval(ctx context.Context, update *ports.BotUpdate, user *domain.User) error {
	log := h.log.With().Str("user_id", user.ID.String()).Logger()
	log.Warn().Msg("User sent text instead of pressing policy buttons")

	// Re-send the policy message
	policyText := "Please accept or decline the policy by pressing the buttons below\\.\n\n[Link to Policy](https://example.com/terms)\n\nDo you accept these terms\\?"

	msg := messages.NewBuilder(update.ChatID).
		WithText(policyText).
		WithInlineButtons([][]ports.Button{
			{
				{Text: "✅ I Accept", Data: "policy_accept"},
				{Text: "❌ I Decline", Data: "policy_decline"},
			},
		}).
		Build()

	return h.bot.SendMessage(ctx, msg)
}

// sendErrorMessage is a helper to send a generic error
func (h *registrationHandler) sendErrorMessage(ctx context.Context, chatID int64, message string) error {
	msgParams := messages.NewBuilder(chatID).
		WithText(message).
		WithParseMode("").Build()
	return h.bot.SendMessage(ctx, msgParams)
}
