package provider

import "context"

// MockProvider is a no-op provider for development and testing.
// It accepts any payment request and returns a synthetic TransactionID.
type MockProvider struct{}

// NewMockProvider creates a new MockProvider instance.
func NewMockProvider() *MockProvider {
	return &MockProvider{}
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

	// Generate a synthetic transaction ID (in real provider, this comes from the provider)
	txnID := "txn_mock_" + req.IdempotencyKey

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

	// In a real provider, unmarshal provider-specific payload here and verify signature
	// For now, just acknowledge the webhook
	return &WebhookEvent{
		EventType:         "payment.completed",
		ProviderEventType: "charge.succeeded",
		Status:            "succeeded",
		ProviderData: map[string]interface{}{
			"provider": "mock",
			"payload_size": len(payload),
		},
	}, nil
}

// Name returns the provider identifier.
func (m *MockProvider) Name() string {
	return "mock"
}
