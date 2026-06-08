package slogext

import (
	"context"
	"log/slog"

	"github.com/victorbecerra/kube-refresh/project-payment-gateway/internal/platform/contextkey"
)

// Ctx returns a logger from the context, or the default logger if not found.
func Ctx(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(contextkey.LoggerKey).(*slog.Logger); ok {
		return logger
	}
	return slog.Default()
}