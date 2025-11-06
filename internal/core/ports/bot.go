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

// --- Bot Client Port (Outbound) ---

// BotClientPort defines the interface for *sending* messages.
// This is the "Adapter" our core logic will call.
type BotClientPort interface {
	SendMessage(ctx context.Context, params SendMessageParams) error
	SetMenuCommands(ctx context.Context, chatID int64, isAdmin bool) error
	// We will add EditMessage, AnswerCallbackQuery, etc. here as needed
}

// --- Bot Handler Port (Inbound) ---

// BotUpdate represents a simplified, generic update.
type BotUpdate struct {
	MessageID    int
	ChatID       int64
	UserID       int64
	Text         string
	Command      string
	CallbackData *string
	Contact      *ContactInfo
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
	Handle(ctx context.Context, update *BotUpdate) error
}

// TextHandler defines the interface for handling any non-command text message.
type TextHandler interface {
	// Handle processes the text message, using the user's state to route logic.
	Handle(ctx context.Context, update *BotUpdate, user *domain.User) error
}
