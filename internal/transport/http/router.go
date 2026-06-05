package http

import (
	"net/http"

	"github.com/victorbecerragit/project-payment-gateway/internal/transport/http/handlers"
)

type Router struct {
	paymentHandler *handlers.PaymentHandler
	healthHandler  *handlers.HealthHandler
}

func NewRouter(p *handlers.PaymentHandler, h *handlers.HealthHandler) *Router {
	return &Router{
		paymentHandler: p,
		healthHandler:  h,
	}
}

// SetupRoutes registers all application routes to the provided mux
func SetupRoutes(mux *http.ServeMux, p *handlers.PaymentHandler, h *handlers.HealthHandler) {
	mux.HandleFunc("GET /health", h.Health)
	mux.HandleFunc("GET /ready", h.Ready)

	mux.HandleFunc("POST /api/v1/payments", p.CreatePayment)
	mux.HandleFunc("GET /api/v1/payments/{payment_id}", p.GetPayment)
	mux.HandleFunc("POST /api/v1/webhooks/payment", p.HandleWebhook)
}

func (r *Router) Handler() http.Handler {
	mux := http.NewServeMux()
	SetupRoutes(mux, r.paymentHandler, r.healthHandler)
	return mux
}
