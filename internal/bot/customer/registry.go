package customer

import (
	"AsaExchange/internal/core/ports"
	"AsaExchange/internal/shared/config"

	"github.com/rs/zerolog"
)

// --- Define types for handler "constructors" ---
// This allows us to pass dependencies from main.go

type CommandHandlerConstructor func(
	cfg *config.Config,
	userRepo ports.UserRepository,
	botClient ports.BotClientPort,
	baseLogger *zerolog.Logger,
) ports.CommandHandler

type CallbackHandlerConstructor func(
	cfg *config.Config,
	userRepo ports.UserRepository,
	botClient ports.BotClientPort,
	baseLogger *zerolog.Logger,
) ports.CallbackHandler

type MessageHandlerConstructor func(
	cfg *config.Config,
	userRepo ports.UserRepository,
	botClient ports.BotClientPort,
	baseLogger *zerolog.Logger,
) ports.MessageHandler

// --- Create the global registries ---
var (
	commandRegistry  []CommandHandlerConstructor
	callbackRegistry []CallbackHandlerConstructor
	messageHandler   MessageHandlerConstructor
)

// RegisterCommand is called by handlers in their init() function
func RegisterCommand(constructor CommandHandlerConstructor) {
	commandRegistry = append(commandRegistry, constructor)
}

// RegisterCallback is called by callback handlers in their init()
func RegisterCallback(constructor CallbackHandlerConstructor) {
	callbackRegistry = append(callbackRegistry, constructor)
}

// RegisterMessage (formerly RegisterText) is called by the message handler
func RegisterMessage(constructor MessageHandlerConstructor) {
	// We only allow one global message handler
	messageHandler = constructor
}

// RegisterAllHandlers is the single function called by main.go
// It builds all registered handlers and passes them to the router.
func RegisterAllHandlers(
	cfg *config.Config,
	router *CustomerRouter,
	userRepo ports.UserRepository,
	botClient ports.BotClientPort,
	baseLogger *zerolog.Logger,
) {
	log := baseLogger.With().Str("component", "customer_registry").Logger()

	// Register all commands
	for _, constructor := range commandRegistry {
		handler := constructor(cfg, userRepo, botClient, baseLogger)
		router.RegisterCommandHandler(handler)
	}

	// Register all callbacks
	for _, constructor := range callbackRegistry {
		handler := constructor(cfg, userRepo, botClient, baseLogger)
		router.RegisterCallbackHandler(handler)
	}

	// Register the single message handler
	if messageHandler != nil {
		// Pass cfg to the constructor
		handler := messageHandler(cfg, userRepo, botClient, baseLogger)
		router.SetMessageHandler(handler)
		log.Info().Msg("Registered main message handler")
	}
}
