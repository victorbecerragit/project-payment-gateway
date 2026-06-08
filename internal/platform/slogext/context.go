package slogext

import (
	"context"
	"log/slog"

	"github.com/victorbecerragit/project-payment-gateway/internal/platform/contextkey"
)

func Ctx(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(contextkey.LoggerKey).(*slog.Logger); ok {
		return logger
	}
	return slog.Default()
}
