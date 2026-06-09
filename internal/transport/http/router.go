package http

import (
	"context"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/victorbecerragit/project-payment-gateway/internal/platform/config"
	"github.com/victorbecerragit/project-payment-gateway/internal/transport/http/handlers"
	"github.com/victorbecerragit/project-payment-gateway/internal/transport/http/middleware"
	"golang.org/x/time/rate"
)

// SetupRoutes registers all application routes to the provided mux
func SetupRoutes(mux *http.ServeMux, p *handlers.PaymentHandler, h *handlers.HealthHandler, m *middleware.RequestMetrics, cfg *config.Config, rateLimiterCtx context.Context) {
	// Initialize rate limiters for different route groups
	apiLimiter := middleware.NewIPRateLimiter(rateLimiterCtx, rate.Limit(cfg.APIRateLimit), cfg.APIBurst, m)
	webhookLimiter := middleware.NewIPRateLimiter(rateLimiterCtx, rate.Limit(cfg.WebhookRateLimit), cfg.WebhookBurst, m)

	// Add rate limiters to the custom Prometheus collector
	m.AddLimiter(apiLimiter, "api")
	m.AddLimiter(webhookLimiter, "webhook")

	// Health routes
	mux.Handle("GET /health", m.MetricsMiddleware("/health", http.HandlerFunc(h.Health)))
	mux.Handle("GET /ready", m.MetricsMiddleware("/ready", http.HandlerFunc(h.Ready)))

	// Payment routes
	mux.Handle("POST /api/v1/payments", apiLimiter.Handler("/api/v1/payments", m.MetricsMiddleware("/api/v1/payments", http.HandlerFunc(p.CreatePayment))))
	mux.Handle("GET /api/v1/payments/{payment_id}", apiLimiter.Handler("/api/v1/payments/{payment_id}", m.MetricsMiddleware("/api/v1/payments/{payment_id}", http.HandlerFunc(p.GetPayment))))

	// Webhook routes use a more permissive limiter to handle bursts from providers
	mux.Handle("POST /api/v1/webhooks/payment", webhookLimiter.Handler("/api/v1/webhooks/payment", m.MetricsMiddleware("/api/v1/webhooks/payment", http.HandlerFunc(p.HandleWebhook))))

	// Prometheus metrics endpoint
	mux.Handle("GET /metrics", promhttp.Handler())
}
