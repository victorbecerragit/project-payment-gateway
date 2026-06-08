package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	apphealth "github.com/victorbecerragit/project-payment-gateway/internal/application/health"
	apppayment "github.com/victorbecerragit/project-payment-gateway/internal/application/payment"
	"github.com/victorbecerragit/project-payment-gateway/internal/domain/payment" // Import domain payment
	"github.com/victorbecerragit/project-payment-gateway/internal/platform/config"
	"github.com/victorbecerragit/project-payment-gateway/internal/provider"
	"github.com/victorbecerragit/project-payment-gateway/internal/provider/stripe"
	"github.com/victorbecerragit/project-payment-gateway/internal/provider/webhook"
	"github.com/victorbecerragit/project-payment-gateway/internal/storage/inmemory"
	"github.com/victorbecerragit/project-payment-gateway/internal/storage/postgres"
	transport "github.com/victorbecerragit/project-payment-gateway/internal/transport/http"
	"github.com/victorbecerragit/project-payment-gateway/internal/transport/http/handlers"
)

func main() {
	cfg := config.Load()

	// Initialize domain-level configurations
	payment.SetSupportedCurrencies(cfg.SupportedCurrencies)

	// Initialize Repositories
	var paymentRepo payment.Repository
	if cfg.DatabaseURL != "" {
		paymentRepo = postgres.NewRepository(context.Background(), cfg.DatabaseURL)
		log.Println("Using Postgres repository")
	} else {
		paymentRepo = inmemory.NewRepository()
		log.Println("Using in-memory repository")
	}

	// Initialize Provider based on configuration flag
	var paymentProvider provider.Provider
	if cfg.StripeAPIKey != "" {
		paymentProvider = stripe.NewStripeProvider(stripe.Config{
			APIKey:        cfg.StripeAPIKey,
			WebhookSecret: cfg.StripeWebhookSecret,
		})
		log.Println("Using Stripe payment provider")
	} else {
		paymentProvider = provider.NewMockProvider()
		log.Println("Using mock payment provider")
	}

	// Initialize Services
	healthService := apphealth.NewService()
	paymentService := apppayment.NewService(paymentRepo, paymentProvider)
	webhookVerifier := webhook.NewMockVerifier()

	// Initialize Handlers
	healthHandler := handlers.NewHealthHandler(healthService)
	paymentHandler := handlers.NewPaymentHandler(paymentService, webhookVerifier)

	mux := http.NewServeMux()

	// Setup routes using the new router
	transport.SetupRoutes(mux, paymentHandler, healthHandler)

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: mux,
	}

	// Run server in a goroutine
	go func() {
		log.Printf("Starting payment gateway server on port %s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Closing repository...")
	paymentRepo.Close()

	log.Println("Server exited gracefully")
}
