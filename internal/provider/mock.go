package provider

import (
	"context"

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

	ctx, span := m.tracer.StartSpan(ctx, "mock.CreatePayment")
	defer span.End()

	// Generate a synthetic transaction ID (in real provider, this comes from the provider)
	txnID := "txn_mock_" + req.IdempotencyKey

	return &CreatePaymentResponse{
		TransactionID: txnID,
		ProviderStatus: "succeeded",
		ProviderData: map[string]interface{}{
			"provider":          "mock",
			"idempotency_key":   req.IdempotencyKey,
		},
	}
	span.SetAttribute("payment.id", req.PaymentID)
	span.SetAttribute("provider.transaction_id", txnID)
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

	ctx, span := m.tracer.StartSpan(ctx, "mock.ParseWebhook")
	defer span.End()

	// In a real provider, unmarshal provider-specific payload here and verify signature
	// For now, just acknowledge the webhook
	return &WebhookEvent{
		EventType:         "payment.completed",
		ProviderEventType: "charge.succeeded",
		Status:            "succeeded",
		ProviderData: map[string]interface{}{
			"provider": "mock",
			"payload_size": len(payload),
			"webhook.signature", signature,
			"webhook.payload", string(payload),
		},
	}, nil
}

// Name returns the provider identifier.
func (m *MockProvider) Name() string {
	return "mock"
}
