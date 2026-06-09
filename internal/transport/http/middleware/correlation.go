package middleware

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	
	"github.com/victorbecerragit/project-payment-gateway/internal/platform/contextkey"
	"github.com/victorbecerragit/project-payment-gateway/internal/platform/tracing"
)

func CorrelationIDMiddleware(tracer tracing.Tracer, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}

		ctx := r.Context()
		ctx = context.WithValue(ctx, contextkey.RequestIDKey, requestID)

		requestLogger := slog.Default().With("request_id", requestID)
		ctx = context.WithValue(ctx, contextkey.LoggerKey, requestLogger)

		ctx, span := tracer.StartSpan(ctx, "http.request")
		defer span.End()

		span.SetAttribute("http.method", r.Method)
		span.SetAttribute("http.path", r.URL.Path)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
