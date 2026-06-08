package payment

import (
	"context"
	"errors"
)

// Service defines the domain operations for payments
type Service interface {
	CreatePayment(ctx context.Context, p *Payment) error
	GetPayment(ctx context.Context, id string) (*Payment, error)
	ProcessEvent(ctx context.Context, e *PaymentEvent) error
}

// ErrPaymentNotFound is returned when a payment cannot be found in the repository.
var ErrPaymentNotFound = errors.New("payment not found")

// Repository defines the storage operations for payments
type Repository interface {
	Save(ctx context.Context, p *Payment) error
	GetByID(ctx context.Context, id string) (*Payment, error)
	GetByIdempotencyKey(ctx context.Context, key string) (*Payment, error)
	// GetByProviderRef looks up a payment by its external provider reference
	// (e.g. Stripe PaymentIntent ID stored in TransactionID). Used as a
	// fallback when a webhook does not carry the internal payment ID in metadata.
	GetByProviderRef(ctx context.Context, providerRef string) (*Payment, error)
}
