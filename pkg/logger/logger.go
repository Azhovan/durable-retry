// Package logger provides a simple wrapper around the slog library
// for structured logging. It offers functions to create a new slog.Logger
// with customizable output destinations and log levels.
package logger

import (
	"io"
	"log/slog"
	"os"
)

// DefaultLogger creates a new logger with default settings.
// It writes logs to standard output (os.Stdout) and uses Info level for logging.
// This function is useful for quick setup of logging with sensible defaults.
func DefaultLogger() *slog.Logger {
	return NewLogger(os.Stdout, &slog.HandlerOptions{
		Level:       slog.LevelInfo,
		ReplaceAttr: removeTime,
	})
}

// NewLogger creates a new slog.Logger with the specified output destination and log level.
func NewLogger(out io.Writer, options *slog.HandlerOptions) *slog.Logger {
	return slog.New(slog.NewTextHandler(out, options))
}

// removeTime is a callback function used in NewLogger to manipulate log attributes.
// It removes the time attribute from the log entries if no specific group is specified.
func removeTime(groups []string, attr slog.Attr) slog.Attr {
	if attr.Key == slog.TimeKey && len(groups) == 0 {
		return slog.Attr{}
	}
	return attr
}
