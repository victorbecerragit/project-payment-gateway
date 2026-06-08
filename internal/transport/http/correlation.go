package middleware

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/victorbecerra/kube-refresh/project-payment-gateway/internal/platform/contextkey"
	"github.com/victorbecerra/kube-refresh/project-payment-gateway/internal/platform/tracing"
)

// CorrelationIDMiddleware generates or propagates a request ID and adds it to the context and logger.
func CorrelationIDMiddleware(tracer tracing.Tracer, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}

		ctx := r.Context()
		ctx = context.WithValue(ctx, contextkey.RequestIDKey, requestID)

		// Create a context-aware logger with the request ID
		requestLogger := slog.Default().With("request_id", requestID)
		ctx = context.WithValue(ctx, contextkey.LoggerKey, requestLogger)

		// Start the root span for this request
		ctx, span := tracer.StartSpan(ctx, "http.request")
		defer span.End()

		// Add request method and path to the span
		span.SetAttribute("http.method", r.Method)
		span.SetAttribute("http.path", r.URL.Path)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}