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

// ... add MessageHandler, CallbackHandler constructors later ...

var (
	commandRegistry []CommandHandlerConstructor
)

func RegisterCommand(constructor CommandHandlerConstructor) {
	commandRegistry = append(commandRegistry, constructor)
}

// RegisterAllHandlers builds all registered moderator handlers
func RegisterAllHandlers(
	cfg *config.Config,
	router *ModeratorRouter,
	userRepo ports.UserRepository,
	botClient ports.BotClientPort,
	baseLogger *zerolog.Logger,
) {
	// Register all commands
	for _, constructor := range commandRegistry {
		handler := constructor(cfg, userRepo, botClient, baseLogger)
		router.RegisterCommandHandler(handler)
	}
}
