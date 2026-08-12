package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"strings"
)

type ctxKey string

const loggerKey ctxKey = "slog_logger"

func LoggingMiddleware(parentLogger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Generate a unique request ID (or pull from headers)
		reqID := generateRequestID()

		// Create a child logger pinned with this request's metadata
		reqLogger := parentLogger.With(
			"request_id", reqID,
			"method", r.Method,
			"path", r.URL.Path,
		)

		// Inject the child logger into the request context
		ctx := context.WithValue(r.Context(), loggerKey, reqLogger)

		reqLogger.Info("Request started")

		// Pass the enriched context down the chain
		next.ServeHTTP(w, r.WithContext(ctx))

		reqLogger.Info("Request completed")
	})
}

func LoggerFromContext(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(loggerKey).(*slog.Logger); ok {
		return logger
	}
	return slog.Default() // Fallback if context doesn't have it
}

func ParseLogLevel(levelStr string) slog.Level {
	switch strings.ToUpper(levelStr) {
	case "DEBUG":
		return slog.LevelDebug
	case "WARN", "WARNING":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo // Safe fallback
	}
}

func generateRequestID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
