// Package logging configures structured process logging.
package logging

import (
	"log/slog"
	"os"
	"strings"
)

// Setup configures the default slog logger for the process.
func Setup(appEnv string) {
	level := slog.LevelInfo
	if strings.EqualFold(appEnv, "local") || strings.EqualFold(appEnv, "development") {
		level = slog.LevelDebug
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(handler))
}
