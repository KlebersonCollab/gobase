package logger

import (
	"io"
	"log/slog"
	"os"
)

func InitLogger(out io.Writer, level string) *slog.Logger {
	var l slog.Level
	switch level {
	case "debug":
		l = slog.LevelDebug
	case "info":
		l = slog.LevelInfo
	case "warn":
		l = slog.LevelWarn
	case "error":
		l = slog.LevelError
	default:
		l = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: l,
	}

	handler := slog.NewJSONHandler(out, opts)
	logger := slog.New(handler)

	// Set default logger globally
	slog.SetDefault(logger)

	return logger
}

func Setup() {
	InitLogger(os.Stdout, "info")
}
