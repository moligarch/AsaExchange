package bot

import (
	"AsaExchange/internal/adapters/telegram"
	"AsaExchange/internal/core/ports"
	"github.com/rs/zerolog"
)

// --- Define types for handler "constructors" ---
// This allows us to pass dependencies from main.go

type CommandHandlerConstructor func(ports.UserRepository, ports.BotClientPort, *zerolog.Logger) ports.CommandHandler
type TextHandlerConstructor func(ports.UserRepository, ports.BotClientPort, *zerolog.Logger) ports.TextHandler

// --- Create the global registries ---

var (
	commandRegistry []CommandHandlerConstructor
	textHandler     TextHandlerConstructor
)

// RegisterCommand is called by handlers in their init() function
func RegisterCommand(constructor CommandHandlerConstructor) {
	commandRegistry = append(commandRegistry, constructor)
}

// RegisterText is called by the text handler in its init() function
func RegisterText(constructor TextHandlerConstructor) {
	// We only allow one global text handler
	textHandler = constructor
}

// RegisterAllHandlers is the single function called by main.go
// It builds all registered handlers and passes them to the router.
func RegisterAllHandlers(
	router *telegram.Router,
	userRepo ports.UserRepository,
	botClient ports.BotClientPort,
	baseLogger *zerolog.Logger,
) {
	log := baseLogger.With().Str("component", "handler_registry").Logger()

	// Register all commands
	for _, constructor := range commandRegistry {
		handler := constructor(userRepo, botClient, baseLogger)
		router.RegisterCommandHandler(handler)
	}

	// Register the single text handler
	if textHandler != nil {
		handler := textHandler(userRepo, botClient, baseLogger)
		router.SetTextHandler(handler)
		log.Info().Msg("Registered main text handler")
	}
}