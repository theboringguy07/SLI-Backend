package logger

import (
	"context"
	"log/slog"
	"os"
)

// Setup initializes the global structured logger.
func Setup() {
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}
	handler := slog.NewJSONHandler(os.Stdout, opts)
	logger := slog.New(handler)
	slog.SetDefault(logger)
}

// Context keys for logging
type contextKey string

const (
	RequestIDKey contextKey = "request_id"
	ActorIDKey   contextKey = "actor_id"
	ActorNameKey contextKey = "actor_name"
)

// WithContext adds context values to the logger.
func WithContext(ctx context.Context) *slog.Logger {
	logger := slog.Default()
	
	if reqID, ok := ctx.Value(RequestIDKey).(string); ok {
		logger = logger.With("request_id", reqID)
	}
	
	if actorID, ok := ctx.Value(ActorIDKey).(string); ok {
		logger = logger.With("actor_id", actorID)
	}

	if actorName, ok := ctx.Value(ActorNameKey).(string); ok {
		logger = logger.With("actor_name", actorName)
	}

	return logger
}
