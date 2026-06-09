package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/victorbecerragit/project-payment-gateway/internal/platform/config"
	"github.com/victorbecerragit/project-payment-gateway/internal/platform/tracing"

	apphealth "github.com/victorbecerragit/project-payment-gateway/internal/application/health"
	apppayment "github.com/victorbecerragit/project-payment-gateway/internal/application/payment"
	"github.com/victorbecerragit/project-payment-gateway/internal/domain/payment"
	"github.com/victorbecerragit/project-payment-gateway/internal/provider"
	"github.com/victorbecerragit/project-payment-gateway/internal/provider/webhook"
	"github.com/victorbecerragit/project-payment-gateway/internal/storage/inmemory"
	gatewayhttp "github.com/victorbecerragit/project-payment-gateway/internal/transport/http"
	"github.com/victorbecerragit/project-payment-gateway/internal/transport/http/dto"
	"github.com/victorbecerragit/project-payment-gateway/internal/transport/http/handlers"
	"github.com/victorbecerragit/project-payment-gateway/internal/transport/http/middleware"
)

// TestPaymentFlow_Integration performs a full payment lifecycle integration test.
// It creates a payment, verifies its pending status, simulates a webhook to complete it,
// and then verifies the completed status and transaction ID.
func TestPaymentFlow_Integration(t *testing.T) {
	// 1. Setup dependencies and router
	tracer := tracing.NewNoOpTracer() // Use no-op tracer for integration tests
	repo := inmemory.NewRepository(tracer)
	paymentProvider := provider.NewMockProvider(tracer)
	paymentSvc := apppayment.NewService(repo, paymentProvider, tracer)
	healthSvc := apphealth.NewService()
	verifier := webhook.NewMockVerifier()
	requestMetrics := middleware.NewRequestMetrics()

	paymentHandler := handlers.NewPaymentHandler(paymentSvc, verifier)
	healthHandler := handlers.NewHealthHandler(healthSvc)

	mux := http.NewServeMux()

	// Temporarily set supported currencies for this test
	originalSupportedCurrencies := payment.GetSupportedCurrencies()
	originalCurrencyStrs := make([]string, 0, len(originalSupportedCurrencies))
	for k := range originalSupportedCurrencies {
		originalCurrencyStrs = append(originalCurrencyStrs, string(k))
	}
	payment.SetSupportedCurrencies([]string{"USD", "EUR", "GBP"}) // Set a default set for tests

	// Dummy config and context for integration test
	dummyConfig := &config.Config{
		APIRateLimit:     100,
		APIBurst:         200,
		WebhookRateLimit: 100,
		WebhookBurst:     200,
	}
	gatewayhttp.SetupRoutes(mux, paymentHandler, healthHandler, requestMetrics, dummyConfig, context.Background())

	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := ts.Client()

	defer func() { payment.SetSupportedCurrencies(originalCurrencyStrs) }()
	// 2. Create Payment (POST /api/v1/payments)
	paymentReq := dto.PaymentRequest{
		Amount:      150.75,
		Currency:    "EUR",
		CustomerID:  "cust_789",
		Description: "Order #456",
	}
	reqBody, _ := json.Marshal(paymentReq)
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/payments", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Idempotency-Key", "integration-test-key")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to execute create payment request: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Expected 201 Created, got %d", resp.StatusCode)
	}

	var paymentCreated dto.PaymentResponse
	if err := json.NewDecoder(resp.Body).Decode(&paymentCreated); err != nil {
		t.Fatalf("Failed to decode payment response: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	paymentID := paymentCreated.PaymentID
	if paymentID == "" {
		t.Fatal("Payment ID should not be empty")
	}

	// 3. Verify Status is Pending (GET /api/v1/payments/{payment_id})
	resp, err = client.Get(ts.URL + "/api/v1/payments/" + paymentID)
	if err != nil {
		t.Fatalf("Failed to execute get payment request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", resp.StatusCode)
	}

	var paymentDetails dto.PaymentResponse
	if err := json.NewDecoder(resp.Body).Decode(&paymentDetails); err != nil {
		t.Fatalf("Failed to decode payment details: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if paymentDetails.Status != "pending" {
		t.Errorf("Expected initial status 'pending', got '%s'", paymentDetails.Status)
	}

	// 4. Handle Webhook - Complete Payment (POST /api/v1/webhooks/payment)
	webhookPayload := dto.WebhookPayload{
		EventType:     "payment.completed",
		PaymentID:     paymentID,
		TransactionID: "tx_integration_001",
		Timestamp:     time.Now().UTC(),
	}
	webhookBody, _ := json.Marshal(webhookPayload)
	req, _ = http.NewRequest(http.MethodPost, ts.URL+"/api/v1/webhooks/payment", bytes.NewBuffer(webhookBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Signature", "valid-signature") // MockVerifier accepts anything but "invalid"

	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("Failed to execute webhook request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 OK for webhook, got %d", resp.StatusCode)
	}
	resp.Body.Close() //nolint:errcheck

	// 5. Verify Status is Completed (GET /api/v1/payments/{payment_id})
	resp, err = client.Get(ts.URL + "/api/v1/payments/" + paymentID)
	if err != nil {
		t.Fatalf("Failed to get final payment status: %v", err)
	}
	if err := json.NewDecoder(resp.Body).Decode(&paymentDetails); err != nil {
		t.Fatalf("Failed to decode final payment details: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if paymentDetails.Status != "completed" {
		t.Errorf("Expected status 'completed' after webhook, got '%s'", paymentDetails.Status)
	}
	if paymentDetails.TransactionID != "tx_integration_001" {
		t.Errorf("Expected TransactionID 'tx_integration_001', got '%s'", paymentDetails.TransactionID)
	}
}