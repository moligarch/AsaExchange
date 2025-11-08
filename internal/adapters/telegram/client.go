package telegram

import (
	"AsaExchange/internal/core/ports"
	"context"
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/rs/zerolog"
)

var _ ports.BotClientPort = (*tgClient)(nil) // Ensure compliance

// tgClient implements the BotClientPort.
type tgClient struct {
	api *tgbotapi.BotAPI
	log zerolog.Logger
}

// NewClient creates a new Telegram client adapter.
func NewClient(api *tgbotapi.BotAPI, baseLogger *zerolog.Logger) ports.BotClientPort {
	log := baseLogger.With().Str("component", "tg_client").Logger()
	return &tgClient{api: api, log: log}
}

// SendMessage translates our params into a tgbotapi message.
func (c *tgClient) SendMessage(ctx context.Context, params ports.SendMessageParams) (messageID int, err error) {
	// We use tgbotapi.Chattable so we can set ReplyMarkup
	msg := tgbotapi.NewMessage(params.ChatID, params.Text)
	msg.ParseMode = params.ParseMode

	// Handle keyboard removal first
	if params.RemoveKeyboard {
		msg.ReplyMarkup = tgbotapi.ReplyKeyboardRemove{RemoveKeyboard: true}
	} else if params.ReplyMarkup != nil {
		if params.ReplyMarkup.IsInline {
			msg.ReplyMarkup = c.buildInlineKeyboard(params.ReplyMarkup.Buttons)
		} else {
			msg.ReplyMarkup = c.buildReplyKeyboard(params.ReplyMarkup.Buttons)
		}
	}

	sentMessage, err := c.api.Send(msg)
	if err != nil {
		c.log.Error().Err(err).Int64("chat_id", params.ChatID).Msg("Failed to send message")
		return 0, err
	}
	return sentMessage.MessageID, nil
}

// buildInlineKeyboard is a helper to create the inline keyboard.
func (c *tgClient) buildInlineKeyboard(buttons [][]ports.Button) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton
	for _, buttonRow := range buttons {
		var row []tgbotapi.InlineKeyboardButton
		for _, btn := range buttonRow {
			if btn.URL != "" {
				row = append(row, tgbotapi.NewInlineKeyboardButtonURL(btn.Text, btn.URL))
			} else {
				row = append(row, tgbotapi.NewInlineKeyboardButtonData(btn.Text, btn.Data))
			}
		}
		rows = append(rows, row)
	}
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// buildReplyKeyboard is a helper to create the reply (non-inline) keyboard.
func (c *tgClient) buildReplyKeyboard(buttons [][]ports.Button) tgbotapi.ReplyKeyboardMarkup {
	var rows [][]tgbotapi.KeyboardButton
	for _, buttonRow := range buttons {
		var row []tgbotapi.KeyboardButton
		for _, btn := range buttonRow {
			if btn.RequestContact {
				row = append(row, tgbotapi.NewKeyboardButtonContact(btn.Text))
			} else {
				row = append(row, tgbotapi.NewKeyboardButton(btn.Text))
			}
		}
		rows = append(rows, row)
	}

	markup := tgbotapi.NewReplyKeyboard(rows...)
	markup.ResizeKeyboard = true
	markup.OneTimeKeyboard = true // Keyboard hides after one use
	return markup
}

// SetMenuCommands sets the bot's /menu commands.
func (c *tgClient) SetMenuCommands(ctx context.Context, chatID int64, isAdmin bool) error {
	var commands []tgbotapi.BotCommand
	if isAdmin {
		commands = []tgbotapi.BotCommand{
			{Command: "/pending", Description: "View pending transactions"},
		}
	} else {
		commands = []tgbotapi.BotCommand{
			{Command: "/start", Description: "Start the bot"},
			{Command: "/newrequest", Description: "Create a new exchange request"},
			{Command: "/myaccounts", Description: "Manage your payout accounts"},
		}
	}

	config := tgbotapi.NewSetMyCommands(commands...)
	if _, err := c.api.Request(config); err != nil {
		c.log.Error().Err(err).Msg("Failed to set menu commands")
		return err
	}
	return nil
}

// EditMessageText edits an existing message (usually for inline keyboards).
func (c *tgClient) EditMessageText(ctx context.Context, params ports.EditMessageParams) error {
	// Create the edit message config
	msg := tgbotapi.NewEditMessageText(
		params.ChatID,
		params.MessageID,
		params.Text,
	)
	msg.ParseMode = params.ParseMode

	// Add new inline keyboard if one is provided
	if params.ReplyMarkup != nil && params.ReplyMarkup.IsInline {
		inlineMarkup := c.buildInlineKeyboard(params.ReplyMarkup.Buttons)
		msg.ReplyMarkup = &inlineMarkup
	}

	// Send the request
	if _, err := c.api.Send(msg); err != nil {
		c.log.Error().Err(err).
			Int64("chat_id", params.ChatID).
			Int("message_id", params.MessageID).
			Msg("Failed to edit message text")
		return err
	}
	return nil
}

// EditMessageCaption edits the caption of an existing media message.
func (c *tgClient) EditMessageCaption(ctx context.Context, params ports.EditMessageCaptionParams) error {
	msg := tgbotapi.NewEditMessageCaption(
		params.ChatID,
		params.MessageID,
		params.Caption,
	)
	msg.ParseMode = params.ParseMode

	if params.ReplyMarkup != nil && params.ReplyMarkup.IsInline {
		inlineMarkup := c.buildInlineKeyboard(params.ReplyMarkup.Buttons)
		msg.ReplyMarkup = &inlineMarkup
	}

	if _, err := c.api.Send(msg); err != nil {
		c.log.Error().Err(err).
			Int64("chat_id", params.ChatID).
			Int("message_id", params.MessageID).
			Msg("Failed to edit message caption")
		return err
	}
	return nil
}

// AnswerCallbackQuery sends a response to a callback query (stops the spinner)
func (c *tgClient) AnswerCallbackQuery(ctx context.Context, params ports.AnswerCallbackParams) error {
	callbackConfig := tgbotapi.NewCallback(params.CallbackQueryID, params.Text)
	callbackConfig.ShowAlert = params.ShowAlert

	if _, err := c.api.Request(callbackConfig); err != nil {
		c.log.Error().Err(err).
			Str("callback_query_id", params.CallbackQueryID).
			Msg("Failed to answer callback query")
		return err
	}
	return nil
}

// SendPhoto sends a photo with a caption and optional keyboard
func (c *tgClient) SendPhoto(ctx context.Context, params ports.SendPhotoParams) (messageID int, err error) {
	var file tgbotapi.RequestFileData
	if filePath, ok := params.File.(string); ok {
		file = tgbotapi.FilePath(filePath)
	} else if fileID, ok := params.File.(tgbotapi.FileID); ok {
		file = fileID
	} else {
		return 0, fmt.Errorf("invalid file type for SendPhoto: %T", params.File)
	}

	photoConfig := tgbotapi.NewPhoto(params.ChatID, file)
	photoConfig.Caption = params.Caption
	photoConfig.ParseMode = params.ParseMode

	if params.ReplyMarkup != nil && params.ReplyMarkup.IsInline {
		photoConfig.ReplyMarkup = c.buildInlineKeyboard(params.ReplyMarkup.Buttons)
	}

	sentMessage, err := c.api.Send(photoConfig)
	if err != nil {
		c.log.Error().Err(err).
			Int64("chat_id", params.ChatID).
			Msg("Failed to send photo")
		return 0, err
	}
	return sentMessage.MessageID, nil
}
