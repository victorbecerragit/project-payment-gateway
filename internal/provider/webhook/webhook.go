package webhook

import "context"

// MockVerifier is a no-op webhook signature verifier for development.
// It accepts any webhook signature as valid.
type MockVerifier struct{}

// NewMockVerifier creates a new MockVerifier instance.
func NewMockVerifier() *MockVerifier {
	return &MockVerifier{}
}

// Verify accepts any webhook as valid (no-op for development).
func (m *MockVerifier) Verify(ctx context.Context, payload []byte, signature string) error {
	// In development, accept all webhooks
	// In production, implement actual signature verification (e.g., HMAC-SHA256 for Stripe)
	return nil
}
