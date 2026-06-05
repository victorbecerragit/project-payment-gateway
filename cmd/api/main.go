package main

import (
	"log"
	"net/http"
	"os"

	"github.com/victorbecerragit/project-payment-gateway/internal/handlers"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()

	// Health check endpoints
	mux.HandleFunc("/health", handlers.HealthHandler)
	mux.HandleFunc("/ready", handlers.ReadyHandler)

	// Payment API endpoints
	mux.HandleFunc("/api/v1/payments", handlers.PaymentHandler)
	mux.HandleFunc("/api/v1/payments/status", handlers.PaymentStatusHandler)
	mux.HandleFunc("/api/v1/webhooks/payment", handlers.WebhookHandler)

	log.Printf("Starting payment gateway server on port %s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
