package payment

import (
	"context"
	"time"
)

// EventType represents a domain event type
type EventType string

const (
	EventPaymentCompleted EventType = "payment.completed"
	EventPaymentFailed    EventType = "payment.failed"
	EventPaymentCancelled EventType = "payment.cancelled"
	// Add other event types here as needed
	// EventPaymentRefunded EventType = "payment.refunded"
	// EventPaymentCaptured EventType = "payment.captured"
)

// PaymentEvent is a normalized domain event representing an external update (e.g., from a webhook)
type PaymentEvent struct {
	Type          EventType
	PaymentID     string
	TransactionID string
	Timestamp     time.Time
}

// WebhookVerifier defines the interface for verifying webhook signatures
type WebhookVerifier interface {
	Verify(ctx context.Context, payload []byte, signature string) error
}