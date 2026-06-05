package payment

import (
	"context"
	"fmt"
	"time"

	"github.com/victorbecerragit/project-payment-gateway/internal/domain/payment"
	"github.com/victorbecerragit/project-payment-gateway/internal/models"
	"github.com/victorbecerragit/project-payment-gateway/internal/platform/id"
)

type service struct {
	repo payment.Repository
}

func NewService(repo payment.Repository) payment.Service {
	return &service{
		repo: repo,
	}
}

func (s *service) CreatePayment(ctx context.Context, p *models.Payment) error {
	if p.Amount <= 0 {
		return fmt.Errorf("amount must be greater than zero")
	}

	// Simple idempotency check
	if p.IdempotencyKey != "" {
		existing, err := s.repo.GetByIdempotencyKey(ctx, p.IdempotencyKey)
		if err == nil && existing != nil {
			*p = *existing // Return existing payment
			return nil
		}
	}

	p.ID = id.GeneratePaymentID()
	p.TransactionID = id.GenerateTransactionID()
	p.Status = "pending"
	p.CreatedAt = time.Now()
	p.UpdatedAt = time.Now()

	return s.repo.Save(ctx, p)
}

func (s *service) GetPayment(ctx context.Context, id string) (*models.Payment, error) {
	return s.repo.GetByID(ctx, id)
}
