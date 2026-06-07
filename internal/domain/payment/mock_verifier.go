package webhook

import (
	"context"
	"errors"
)

// MockVerifier implements the payment.WebhookVerifier interface for testing/development
type MockVerifier struct{}

func NewMockVerifier() *MockVerifier {
	return &MockVerifier{}
}

func (v *MockVerifier) Verify(ctx context.Context, payload []byte, signature string) error {
	if signature == "invalid" {
		return errors.New("invalid webhook signature")
	}
	return nil
}