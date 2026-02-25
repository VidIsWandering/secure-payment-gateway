package logger

import (
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
)

// New creates a configured zerolog.Logger.
// level: debug, info, warn, error. pretty: human-readable console output.
func New(level string, pretty bool) zerolog.Logger {
	var w io.Writer = os.Stdout

	if pretty {
		w = zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
		}
	}

	lvl := parseLevel(level)

	return zerolog.New(w).
		Level(lvl).
		With().
		Timestamp().
		Caller().
		Logger()
}

// NewWithWriter creates a logger writing to a custom writer (useful for testing).
func NewWithWriter(level string, w io.Writer) zerolog.Logger {
	lvl := parseLevel(level)
	return zerolog.New(w).
		Level(lvl).
		With().
		Timestamp().
		Logger()
}

func parseLevel(level string) zerolog.Level {
	switch level {
	case "debug":
		return zerolog.DebugLevel
	case "info":
		return zerolog.InfoLevel
	case "warn":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	default:
		return zerolog.InfoLevel
	}
}
