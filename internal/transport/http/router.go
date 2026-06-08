package http

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/victorbecerra/kube-refresh/project-payment-gateway/internal/transport/http/middleware"
	"github.com/victorbecerragit/project-payment-gateway/internal/transport/http/handlers"
)

type Router struct {
	paymentHandler *handlers.PaymentHandler
	healthHandler  *handlers.HealthHandler
	metrics        *middleware.RequestMetrics
}

func NewRouter(p *handlers.PaymentHandler, h *handlers.HealthHandler, m *middleware.RequestMetrics) *Router {
	return &Router{
		paymentHandler: p,
		healthHandler:  h,
		metrics:        m,
	}
}

// SetupRoutes registers all application routes to the provided mux
func SetupRoutes(mux *http.ServeMux, p *handlers.PaymentHandler, h *handlers.HealthHandler, m *middleware.RequestMetrics) {
	// Health routes
	mux.Handle("GET /health", m.MetricsMiddleware("/health", http.HandlerFunc(h.Health)))
	mux.Handle("GET /ready", m.MetricsMiddleware("/ready", http.HandlerFunc(h.Ready)))

	// Payment routes
	mux.Handle("POST /api/v1/payments", m.MetricsMiddleware("/api/v1/payments", http.HandlerFunc(p.CreatePayment)))
	mux.Handle("GET /api/v1/payments/{payment_id}", m.MetricsMiddleware("/api/v1/payments/{payment_id}", http.HandlerFunc(p.GetPayment)))
	mux.Handle("POST /api/v1/webhooks/payment", m.MetricsMiddleware("/api/v1/webhooks/payment", http.HandlerFunc(p.HandleWebhook)))

	// Prometheus metrics endpoint
	mux.Handle("GET /metrics", promhttp.Handler())
}

func (r *Router) Handler() http.Handler {
	mux := http.NewServeMux() // Create a new mux for the Handler method
	SetupRoutes(mux, r.paymentHandler, r.healthHandler, r.metrics)
	return mux // Return the mux with routes and middleware applied
}
