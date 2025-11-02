package logger

import (
	"os"
	"time"

	"github.com/rs/zerolog"
)

// New initializes a new zerolog.Logger.
// 'devMode' enables human-readable console logging.
func New(devMode bool) zerolog.Logger {
	var logger zerolog.Logger

	if devMode {
		// Human-readable, colorful output for local development
		consoleWriter := zerolog.ConsoleWriter{
			Out:        os.Stderr,
			TimeFormat: time.RFC3339,
		}
		logger = zerolog.New(consoleWriter).With().Timestamp().Logger()
	} else {
		// Efficient JSON output for production
		logger = zerolog.New(os.Stderr).With().Timestamp().Logger()
	}

	return logger
}
