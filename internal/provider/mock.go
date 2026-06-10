package provider

import (
	"context"
	"encoding/json"

	"github.com/victorbecerragit/project-payment-gateway/internal/platform/tracing"
)

// MockProvider is a no-op provider for development and testing.
// It accepts any payment request and returns a synthetic TransactionID.
type MockProvider struct {
	tracer tracing.Tracer
}

// NewMockProvider creates a new MockProvider instance.
func NewMockProvider(tracer tracing.Tracer) *MockProvider {
	if tracer == nil {
		tracer = tracing.NewNoOpTracer()
	}
	return &MockProvider{tracer: tracer}
}

// CreatePayment simulates a successful payment creation without calling any external service.
func (m *MockProvider) CreatePayment(ctx context.Context, req *CreatePaymentRequest) (*CreatePaymentResponse, error) {
	if req == nil {
		return nil, &ErrProviderError{
			Provider: m.Name(),
			Message:  "request is nil",
			Code:     "invalid_request",
		}
	}

	_, span := m.tracer.StartSpan(ctx, "mock.CreatePayment")
	defer span.End()

	// Generate a synthetic transaction ID (in real provider, this comes from the provider)
	txnID := "txn_mock_" + req.IdempotencyKey

	span.SetAttribute("payment.id", req.PaymentID)
	span.SetAttribute("provider.transaction_id", txnID)
	return &CreatePaymentResponse{
		TransactionID: txnID,
		ProviderStatus: "succeeded",
		ProviderData: map[string]interface{}{
			"provider":          "mock",
			"idempotency_key":   req.IdempotencyKey,
		},
	}, nil
}

// ParseWebhook simulates webhook parsing without actual signature verification.
func (m *MockProvider) ParseWebhook(ctx context.Context, payload []byte, signature string) (*WebhookEvent, error) {
	if len(payload) == 0 {
		return nil, &ErrProviderError{
			Provider: m.Name(),
			Message:  "payload is empty",
			Code:     "invalid_payload",
		}
	}

	_, span := m.tracer.StartSpan(ctx, "mock.ParseWebhook")
	defer span.End()

	var parsed struct {
		EventType     string `json:"event_type"`
		PaymentID     string `json:"payment_id"`
		TransactionID string `json:"transaction_id"`
	}

	if err := json.Unmarshal(payload, &parsed); err != nil {
		return nil, &ErrProviderError{
			Provider: m.Name(),
			Message:  "invalid webhook payload",
			Code:     "invalid_payload",
		}
	}

	if parsed.EventType == "" {
		parsed.EventType = "payment.completed"
	}

	span.SetAttribute("webhook.signature", signature)
	span.SetAttribute("webhook.payload", string(payload))
	return &WebhookEvent{
		EventType:         parsed.EventType,
		PaymentID:         parsed.PaymentID,
		TransactionID:     parsed.TransactionID,
		ProviderEventType: "charge.succeeded",
		Status:            "succeeded",
		ProviderData: map[string]interface{}{
			"provider":     "mock",
			"payload_size": len(payload),
		},
	}, nil
}

// Name returns the provider identifier.
func (m *MockProvider) Name() string {
	return "mock"
}
