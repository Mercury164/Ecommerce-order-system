package logger

import (
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

func New(serviceName, level string) zerolog.Logger {
	zerolog.TimeFieldFormat = time.RFC3339Nano

	lvl := zerolog.InfoLevel
	if parsed, err := zerolog.ParseLevel(strings.ToLower(level)); err == nil {
		lvl = parsed
	}

	return zerolog.New(os.Stdout).
		Level(lvl).
		With().
		Timestamp().
		Str("service", serviceName).
		Logger()
}
