package main

import (
	"log"
	"net/http"

	apphealth "github.com/victorbecerragit/project-payment-gateway/internal/application/health"
	apppayment "github.com/victorbecerragit/project-payment-gateway/internal/application/payment"
	"github.com/victorbecerragit/project-payment-gateway/internal/platform/config"
	"github.com/victorbecerragit/project-payment-gateway/internal/provider/webhook"
	"github.com/victorbecerragit/project-payment-gateway/internal/storage/inmemory"
	transport "github.com/victorbecerragit/project-payment-gateway/internal/transport/http"
	"github.com/victorbecerragit/project-payment-gateway/internal/transport/http/handlers"
)

func main() {
	cfg := config.Load()

	// Initialize Repositories
	paymentRepo := inmemory.NewRepository()

	// Initialize Services
	healthService := apphealth.NewService()
	paymentService := apppayment.NewService(paymentRepo)
	webhookVerifier := webhook.NewMockVerifier()

	// Initialize Handlers
	healthHandler := handlers.NewHealthHandler(healthService)
	paymentHandler := handlers.NewPaymentHandler(paymentService, webhookVerifier)

	mux := http.NewServeMux()

	// Setup routes using the new router
	transport.SetupRoutes(mux, paymentHandler, healthHandler)

	log.Printf("Starting payment gateway server on port %s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, mux); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
