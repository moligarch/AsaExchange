package moderator

import (
	"AsaExchange/internal/core/ports"
	"AsaExchange/internal/shared/config"

	"github.com/rs/zerolog"
)

// Define constructor types for moderator handlers
type CommandHandlerConstructor func(
	cfg *config.Config,
	userRepo ports.UserRepository,
	botClient ports.BotClientPort,
	baseLogger *zerolog.Logger,
) ports.CommandHandler

type MessageHandlerConstructor func(
	cfg *config.Config,
	userRepo ports.UserRepository,
	botClient ports.BotClientPort,
	baseLogger *zerolog.Logger,
) ports.MessageHandler

type CallbackHandlerConstructor func(
	cfg *config.Config,
	userRepo ports.UserRepository,
	botClient ports.BotClientPort,
	bus ports.EventBus,
	baseLogger *zerolog.Logger,
) ports.CallbackHandler

var (
	commandRegistry  []CommandHandlerConstructor
	messageHandler   MessageHandlerConstructor
	callbackRegistry []CallbackHandlerConstructor
)

// RegisterCommand
func RegisterCommand(constructor CommandHandlerConstructor) {
	commandRegistry = append(commandRegistry, constructor)
}

// RegisterCallback
func RegisterCallback(constructor CallbackHandlerConstructor) {
	callbackRegistry = append(callbackRegistry, constructor)
}

func RegisterAllHandlers(
	cfg *config.Config,
	router *ModeratorRouter,
	userRepo ports.UserRepository,
	botClient ports.BotClientPort,
	bus ports.EventBus,
	baseLogger *zerolog.Logger,
) {
	log := baseLogger.With().Str("component", "moderator_registry").Logger()
	// Register all commands
	for _, constructor := range commandRegistry {
		handler := constructor(cfg, userRepo, botClient, baseLogger)
		router.RegisterCommandHandler(handler)
	}

	// Register the single message handler
	if messageHandler != nil {
		handler := messageHandler(cfg, userRepo, botClient, baseLogger)
		router.SetMessageHandler(handler)
		log.Info().Msg("Registered main message handler")
	}

	// Register all callbacks
	for _, constructor := range callbackRegistry {
		handler := constructor(cfg, userRepo, botClient, bus, baseLogger)
		router.RegisterCallbackHandler(handler)
	}
}
