package application

import (
	"io"
	"log/slog"

	temporallog "go.temporal.io/sdk/log"
)

// WorkerLogger uses the SDK's slog adapter so workflow messages remain replay-safe.
// Debug-level SDK heartbeats stay out of the normal terminal and service log.
func (app *Service) WorkerLogger() temporallog.Logger {
	output := app.Output
	if output == nil {
		output = io.Discard
	}
	logger := slog.New(slog.NewTextHandler(output, &slog.HandlerOptions{Level: slog.LevelInfo}))
	return temporallog.NewStructuredLogger(logger.With("project", app.Root))
}
