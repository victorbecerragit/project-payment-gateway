package payment

import (
	"context"
)

// Service defines the domain operations for payments
type Service interface {
	CreatePayment(ctx context.Context, p *Payment) error
	GetPayment(ctx context.Context, id string) (*Payment, error)
	ProcessEvent(ctx context.Context, e *PaymentEvent) error
}

// Repository defines the storage operations for payments
type Repository interface {
	Save(ctx context.Context, p *Payment) error
	GetByID(ctx context.Context, id string) (*Payment, error)
	GetByIdempotencyKey(ctx context.Context, key string) (*Payment, error)
}
