package ports

import (
	"AsaExchange/internal/core/domain"
	"context"
)

// --- Bot Message Structures ---

type ContactInfo struct {
	PhoneNumber string
	UserID      int64 // The Telegram ID of the contact
}

type PhotoInfo struct {
	FileID   string // The tgbotapi FileID, which we will store
	FileSize int
}

// Button represents a single button in a keyboard.
type Button struct {
	Text           string
	Data           string // For callbacks
	URL            string // For URL buttons
	RequestContact bool
}

// ReplyMarkup represents any kind of keyboard markup.
type ReplyMarkup struct {
	Buttons  [][]Button
	IsInline bool // Differentiates between Inline and Reply keyboards
}

// SendMessageParams holds all possible options for sending a message.
type SendMessageParams struct {
	ChatID         int64
	Text           string
	ParseMode      string // e.g., "MarkdownV2" or "HTML"
	ReplyMarkup    *ReplyMarkup
	RemoveKeyboard bool
}

// EditMessageParams holds options for editing an existing message.
type EditMessageParams struct {
	ChatID      int64
	MessageID   int
	Text        string
	ParseMode   string
	ReplyMarkup *ReplyMarkup // For inline keyboards
}

// AnswerCallbackParams holds options for answering a callback query.
type AnswerCallbackParams struct {
	CallbackQueryID string
	Text            string // The text for the (optional) pop-up notification
	ShowAlert       bool   // Show as a pop-up alert instead of a toast
}

// --- Bot Client Port (Outbound) ---

// BotClientPort defines the interface for *sending* messages.
// This is the "Adapter" our core logic will call.
type BotClientPort interface {
	SendMessage(ctx context.Context, params SendMessageParams) error
	SetMenuCommands(ctx context.Context, chatID int64, isAdmin bool) error
	// EditMessageText allows us to change the text of an existing message.
	EditMessageText(ctx context.Context, params EditMessageParams) error

	AnswerCallbackQuery(ctx context.Context, params AnswerCallbackParams) error
}

// --- Bot Handler Port (Inbound) ---

// BotUpdate represents a simplified, generic update.
type BotUpdate struct {
	MessageID       int
	ChatID          int64
	UserID          int64
	Text            string
	Command         string
	CallbackQueryID string
	CallbackData    *string
	Contact         *ContactInfo
	Photo           *PhotoInfo
}

// CommandHandler defines the "plugin" interface for handling bot commands.
type CommandHandler interface {
	// Command returns the command string (e.g., "/start")
	Command() string
	// Handle processes the update.
	Handle(ctx context.Context, update *BotUpdate) error
}

// CallbackHandler defines the interface for handling callback queries.
type CallbackHandler interface {
	// Prefix returns the prefix for the callback (e.g., "bid_")
	Prefix() string
	// Handle processes the callback.
	Handle(ctx context.Context, update *BotUpdate, user *domain.User) error
}

// MessageHandler defines the interface
// for handling any message that is not a command or callback.
type MessageHandler interface {
	// Handle processes the message, using the user's state to route logic.
	Handle(ctx context.Context, update *BotUpdate, user *domain.User) error
}
