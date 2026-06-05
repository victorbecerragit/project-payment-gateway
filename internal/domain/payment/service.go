package payment

import (
	"context"

	"github.com/victorbecerragit/project-payment-gateway/internal/models"
)

// Service defines the domain operations for payments
type Service interface {
	CreatePayment(ctx context.Context, p *models.Payment) error
	GetPayment(ctx context.Context, id string) (*models.Payment, error)
}

// Repository defines the storage operations for payments
type Repository interface {
	Save(ctx context.Context, p *models.Payment) error
	GetByID(ctx context.Context, id string) (*models.Payment, error)
	GetByIdempotencyKey(ctx context.Context, key string) (*models.Payment, error)
}
