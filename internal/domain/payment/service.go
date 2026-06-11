package payment

import (
	"context"
	"errors"
)

// Service defines the domain operations for payments
type Service interface {
	CreatePayment(ctx context.Context, p *Payment) error
	GetPayment(ctx context.Context, id string) (*Payment, error)
	ListPayments(ctx context.Context, f ListFilter) ([]*Payment, error)
	ProcessEvent(ctx context.Context, e *PaymentEvent) error
	ParseWebhook(ctx context.Context, payload []byte, signature string) (*PaymentEvent, error)
}
// ErrPaymentNotFound is returned when a payment cannot be found in the repository.
var ErrPaymentNotFound = errors.New("payment not found")

// ErrUnknownEventType is returned when a webhook event type has no domain mapping.
// Callers should acknowledge the webhook (200 OK) and skip processing.
var ErrUnknownEventType = errors.New("unknown event type")

// ListFilter defines optional filters for listing payments.
type ListFilter struct {
	Status string // empty = all statuses
	Limit  int    // 0 = default (50)
}

// Repository defines the storage operations for payments
type Repository interface {
	Save(ctx context.Context, p *Payment) error
	GetByID(ctx context.Context, id string) (*Payment, error)
	GetByIdempotencyKey(ctx context.Context, key string) (*Payment, error)
	GetByProviderRef(ctx context.Context, providerRef string) (*Payment, error)
	ListPayments(ctx context.Context, f ListFilter) ([]*Payment, error)
	Close()
}
