package handlers

import (
	"AsaExchange/internal/core/ports"
	"AsaExchange/internal/shared/config"
	"context"
	"fmt"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/rs/zerolog"
)

// forwardingHandler listens to the queue and forwards to the admin channel
type ForwardingHandler struct {
	log                  zerolog.Logger
	userRepo             ports.UserRepository
	bot                  ports.BotClientPort
	adminReviewChannelID int64
	countryStrategies    map[string]config.CountryConfig
}

// NewForwardingHandler creates a new handler for forwarding verification events
func NewForwardingHandler(
	cfg *config.Config,
	userRepo ports.UserRepository,
	bot ports.BotClientPort,
	baseLogger *zerolog.Logger,
) *ForwardingHandler {
	return &ForwardingHandler{
		log:                  baseLogger.With().Str("component", "forwarding_handler").Logger(),
		userRepo:             userRepo,
		bot:                  bot,
		adminReviewChannelID: cfg.Bot.Moderator.AdminReviewChannelID,
		countryStrategies:    cfg.Bot.Customer.CountryStrategies,
	}
}

// HandleEvent is the method that will be subscribed to the VerificationQueue
func (h *ForwardingHandler) HandleEvent(event ports.NewVerificationEvent) error {
	ctx := context.Background()
	log := h.log.With().Str("user_id", event.UserID.String()).Logger()
	log.Info().Msg("Processing new verification event from queue")

	// 1. Build the inline buttons
	approveData := fmt.Sprintf("approval_accept_%s", event.UserID.String())
	rejectData := fmt.Sprintf("approval_reject_%s", event.UserID.String())

	buttons := [][]ports.Button{
		{
			{Text: "✅ Approve", Data: approveData},
			{Text: "❌ Reject", Data: rejectData},
		},
	}

	// 2. Escape the caption for MarkdownV2
	// The caption from the event is plain text. We re-format it for the admin.
	user, err := h.userRepo.GetByID(ctx, event.UserID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get user for forwarding")
		return err
	}
	if user == nil {
		log.Error().Msg("User not found for forwarding, this should not happen")
		return fmt.Errorf("user %s not found", event.UserID)
	}

	var caption strings.Builder
	caption.WriteString(fmt.Sprintf("*User for Review*\nID: `%s`\n\n", user.ID.String()))
	if user.FirstName != nil {
		caption.WriteString(fmt.Sprintf("*First Name:* %s\n", escapeMarkdown(*user.FirstName)))
	}
	if user.LastName != nil {
		caption.WriteString(fmt.Sprintf("*Last Name:* %s\n", escapeMarkdown(*user.LastName)))
	}
	if user.PhoneNumber != nil {
		caption.WriteString(fmt.Sprintf("*Phone:* `%s`\n", escapeMarkdown(*user.PhoneNumber)))
	}
	if user.GovernmentID != nil {
		caption.WriteString(fmt.Sprintf("*Gov ID:* `%s`\n", escapeMarkdown(*user.GovernmentID)))
	}
	if user.LocationCountry != nil {
		countryTitle := *user.LocationCountry // Fallback to ISO code
		if country, ok := h.countryStrategies[*user.LocationCountry]; ok {
			countryTitle = country.Title
		}
		caption.WriteString(fmt.Sprintf("*Country:* %s\n", escapeMarkdown(countryTitle)))
	}

	// 3. Send the photo (using its FileID) to the *admin review channel*
	photoParams := ports.SendPhotoParams{
		ChatID:    h.adminReviewChannelID,
		File:      tgbotapi.FileID(event.FileID),
		Caption:   caption.String(),
		ParseMode: "MarkdownV2",
		ReplyMarkup: &ports.ReplyMarkup{
			IsInline: true,
			Buttons:  buttons,
		},
	}

	if _, err := h.bot.SendPhoto(ctx, photoParams); err != nil {
		log.Error().Err(err).Msg("Failed to forward verification photo to admin channel")
		return err
	}

	log.Info().Msg("Successfully forwarded verification request to admins")
	return nil
}

func escapeMarkdown(s string) string {
	replacer := strings.NewReplacer(
		"_", "\\_", "*", "\\*", "[", "\\[", "]", "\\]", "(", "\\(", ")", "\\)",
		"~", "\\~", "`", "\\`", ">", "\\>", "#", "\\#", "+", "\\+", "-", "\\-",
		"=", "\\=", "|", "\\|", "{", "\\{", "}", "\\}", ".", "\\.", "!", "\\!",
	)
	return replacer.Replace(s)
}
