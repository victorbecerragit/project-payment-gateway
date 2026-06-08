package stripe_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/victorbecerragit/project-payment-gateway/internal/provider"
	"github.com/victorbecerragit/project-payment-gateway/internal/provider/stripe"
)

func TestStripeProvider_Name(t *testing.T) {
	p := stripe.NewStripeProvider(stripe.Config{APIKey: "sk_test_123"})
	if p.Name() != "stripe" {
		t.Errorf("expected provider name to be 'stripe', got '%s'", p.Name())
	}
}

func TestStripeProvider_CreatePayment_Success(t *testing.T) {
	// Setup local test server to intercept Stripe outbound requests
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request details
		if r.Method != "POST" {
			t.Errorf("expected POST request, got %s", r.Method)
		}
		if r.URL.Path != "/v1/payment_intents" {
			t.Errorf("expected path /v1/payment_intents, got %s", r.URL.Path)
		}

		// Verify Authentication Header matching Stripe requirements
		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer sk_test_key" {
			t.Errorf("expected Authorization header Bearer sk_test_key, got %s", authHeader)
		}

		// Verify Idempotency Header
		idemHeader := r.Header.Get("Idempotency-Key")
		if idemHeader != "idem-key-123" {
			t.Errorf("expected Idempotency-Key header idem-key-123, got %s", idemHeader)
		}

		// Read and verify URL-encoded body
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}
		vals, err := url.ParseQuery(string(bodyBytes))
		if err != nil {
			t.Fatalf("failed to parse url encoded query: %v", err)
		}

		if vals.Get("amount") != "5000" {
			t.Errorf("expected amount 5000, got %s", vals.Get("amount"))
		}
		if vals.Get("currency") != "usd" {
			t.Errorf("expected currency usd, got %s", vals.Get("currency"))
		}
		if vals.Get("metadata[payment_id]") != "pay_abc" {
			t.Errorf("expected metadata[payment_id] pay_abc, got %s", vals.Get("metadata[payment_id]"))
		}

		// Write realistic Stripe PaymentIntent payload
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{
			"id": "pi_12345abcd",
			"object": "payment_intent",
			"status": "requires_payment_method",
			"amount": 5000,
			"currency": "usd",
			"client_secret": "pi_12345abcd_secret_999",
			"metadata": {
				"payment_id": "pay_abc",
				"customer_id": "cust_789"
			}
		}`)
	}))
	defer server.Close()

	// Initialize Provider pointing to local test endpoint
	prov := stripe.NewStripeProvider(stripe.Config{
		APIKey:  "sk_test_key",
		BaseURL: server.URL,
	})

	req := &provider.CreatePaymentRequest{
		PaymentID:      "pay_abc",
		Amount:         5000,
		Currency:       "USD",
		Description:    "Test Payment",
		CustomerID:     "cust_789",
		IdempotencyKey: "idem-key-123",
	}

	resp, err := prov.CreatePayment(context.Background(), req)
	if err != nil {
		t.Fatalf("CreatePayment returned unexpected error: %v", err)
	}

	if resp == nil {
		t.Fatal("expected non-nil response")
	}

	if resp.TransactionID != "pi_12345abcd" {
		t.Errorf("expected transaction ID 'pi_12345abcd', got '%s'", resp.TransactionID)
	}
	if resp.ProviderStatus != "requires_payment_method" {
		t.Errorf("expected status 'requires_payment_method', got '%s'", resp.ProviderStatus)
	}
	stripeID, ok := resp.ProviderData["payment_intent"].(string)
	if !ok || stripeID != "pi_12345abcd" {
		t.Errorf("expected provider target metadata to hold active Stripe intent")
	}
}

func TestStripeProvider_CreatePayment_StripeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `{
			"error": {
				"message": "The card number is incorrect.",
				"type": "card_error",
				"code": "incorrect_number"
			}
		}`)
	}))
	defer server.Close()

	prov := stripe.NewStripeProvider(stripe.Config{
		APIKey:  "sk_test_key",
		BaseURL: server.URL,
	})

	req := &provider.CreatePaymentRequest{
		PaymentID:   "pay_abc",
		Amount:      5000,
		Currency:    "USD",
		CustomerID:  "cust_789",
		Description: "Failed flow",
	}

	_, err := prov.CreatePayment(context.Background(), req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	provErr, ok := err.(*provider.ErrProviderError)
	if !ok {
		t.Fatalf("expected ErrProviderError, got %T", err)
	}

	if provErr.Provider != "stripe" {
		t.Errorf("expected provider name 'stripe' inside error payload, got %s", provErr.Provider)
	}
	if provErr.Code != "incorrect_number" {
		t.Errorf("expected Stripe error code 'incorrect_number', got %s", provErr.Code)
	}
	if !strings.Contains(provErr.Message, "card number is incorrect") {
		t.Errorf("expected explicit message, got '%s'", provErr.Message)
	}
}

func TestStripeProvider_Webhook_VerifyAndParse(t *testing.T) {
	payload := []byte(`{
		"id": "evt_test123",
		"type": "payment_intent.succeeded",
		"data": {
			"object": {
				"id": "pi_123_intent",
				"object": "payment_intent",
				"status": "succeeded",
				"metadata": {
					"payment_id": "pay_abc",
					"idempotency_key": "key-99"
				}
			}
		}
	}`)

	secret := "whsec_test_secret"
	timestamp := "1500000000"

	// Compute Stripe standard HMAC-SHA256 signature
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + "." + string(payload)))
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	sigHeader := fmt.Sprintf("t=%s,v1=%s", timestamp, expectedSig)

	t.Run("Valid Webhook Signature", func(t *testing.T) {
		prov := stripe.NewStripeProvider(stripe.Config{
			APIKey:        "sk_test_key",
			WebhookSecret: secret,
		})

		event, err := prov.ParseWebhook(context.Background(), payload, sigHeader)
		if err != nil {
			t.Fatalf("failed to parse valid webhook: %v", err)
		}

		if event.PaymentID != "pay_abc" {
			t.Errorf("expected paymentID 'pay_abc', got '%s'", event.PaymentID)
		}
		if event.TransactionID != "pi_123_intent" {
			t.Errorf("expected transactionID 'pi_123_intent', got '%s'", event.TransactionID)
		}
		if event.EventType != "payment.completed" {
			t.Errorf("expected event type 'payment.completed', got '%s'", event.EventType)
		}
		if event.Status != "completed" {
			t.Errorf("expected status 'completed', got '%s'", event.Status)
		}
	})

	t.Run("Invalid Webhook Signature", func(t *testing.T) {
		prov := stripe.NewStripeProvider(stripe.Config{
			APIKey:        "sk_test_key",
			WebhookSecret: secret,
		})

		badHeader := sigHeader + "1"
		_, err := prov.ParseWebhook(context.Background(), payload, badHeader)
		if err == nil {
			t.Fatal("expected error with tampered signature, got nil")
		}
	})

	t.Run("Malformed Signature Header", func(t *testing.T) {
		prov := stripe.NewStripeProvider(stripe.Config{
			APIKey:        "sk_test_key",
			WebhookSecret: secret,
		})

		_, err := prov.ParseWebhook(context.Background(), payload, "t=invalid,v1=nonsense")
		if err == nil {
			t.Fatal("expected error with malformed signature header, got nil")
		}
	})
}
