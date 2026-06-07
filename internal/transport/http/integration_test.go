package http_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	apphealth "github.com/victorbecerragit/project-payment-gateway/internal/application/health"
	apppayment "github.com/victorbecerragit/project-payment-gateway/internal/application/payment"
	"github.com/victorbecerragit/project-payment-gateway/internal/provider/webhook"
	"github.com/victorbecerragit/project-payment-gateway/internal/storage/inmemory"
	gatewayhttp "github.com/victorbecerragit/project-payment-gateway/internal/transport/http"
	"github.com/victorbecerragit/project-payment-gateway/internal/transport/http/dto"
	"github.com/victorbecerragit/project-payment-gateway/internal/transport/http/handlers"
)

// TestPaymentFlow_Integration performs a full payment lifecycle integration test.
// It creates a payment, verifies its pending status, simulates a webhook to complete it,
// and then verifies the completed status and transaction ID.
func TestPaymentFlow_Integration(t *testing.T) {
	// 1. Setup dependencies and router
	repo := inmemory.NewRepository()
	paymentSvc := apppayment.NewService(repo)
	healthSvc := apphealth.NewService()
	verifier := webhook.NewMockVerifier()

	paymentHandler := handlers.NewPaymentHandler(paymentSvc, verifier)
	healthHandler := handlers.NewHealthHandler(healthSvc)

	mux := http.NewServeMux()

	// Temporarily set supported currencies for this test
	originalSupportedCurrencies := payment.globalSupportedCurrencies
	payment.SetSupportedCurrencies([]string{"USD", "EUR", "GBP"}) // Set a default set for tests
	gatewayhttp.SetupRoutes(mux, paymentHandler, healthHandler)

	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := ts.Client()

	defer func() { payment.globalSupportedCurrencies = originalSupportedCurrencies }()
	// 2. Create Payment (POST /api/v1/payments)
	paymentReq := dto.PaymentRequest{
		Amount:      150.75,
		Currency:    "EUR",
		CustomerID:  "user_789",
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
	resp.Body.Close()

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
	json.NewDecoder(resp.Body).Decode(&paymentDetails)
	resp.Body.Close()

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
	resp.Body.Close()

	// 5. Verify Status is Completed (GET /api/v1/payments/{payment_id})
	resp, err = client.Get(ts.URL + "/api/v1/payments/" + paymentID)
	json.NewDecoder(resp.Body).Decode(&paymentDetails)
	resp.Body.Close()

	if paymentDetails.Status != "completed" {
		t.Errorf("Expected status 'completed' after webhook, got '%s'", paymentDetails.Status)
	}
	if paymentDetails.TransactionID != "tx_integration_001" {
		t.Errorf("Expected TransactionID 'tx_integration_001', got '%s'", paymentDetails.TransactionID)
	}
}