package log

import (
	"io"
	"os"
	"strings"

	"github.com/rs/zerolog"
)

// Logger wraps zerolog.Logger
type Logger struct {
	*zerolog.Logger
}

// New creates a new logger with the specified level
func New(level string, output io.Writer) *Logger {
	if output == nil {
		output = os.Stdout
	}

	// Parse log level
	zlevel := parseLevel(level)
	zerolog.SetGlobalLevel(zlevel)

	// Create logger with timestamp and caller
	zl := zerolog.New(output).
		With().
		Timestamp().
		Caller().
		Logger()

	return &Logger{Logger: &zl}
}

// NewConsole creates a human-friendly console logger
func NewConsole(level string) *Logger {
	output := zerolog.ConsoleWriter{Out: os.Stdout}
	return New(level, &output)
}

func parseLevel(level string) zerolog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return zerolog.DebugLevel
	case "info":
		return zerolog.InfoLevel
	case "warn", "warning":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	case "fatal":
		return zerolog.FatalLevel
	case "panic":
		return zerolog.PanicLevel
	default:
		return zerolog.InfoLevel
	}
}
