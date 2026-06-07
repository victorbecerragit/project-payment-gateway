package apppayment

import (
	"context"
	"fmt"

	"github.com/victorbecerragit/project-payment-gateway/internal/domain/payment"
	"github.com/victorbecerragit/project-payment-gateway/internal/platform/id"
)

type Service interface {
	CreatePayment(ctx context.Context, p *payment.Payment) error
	GetPayment(ctx context.Context, paymentID string) (*payment.Payment, error)
	ProcessEvent(ctx context.Context, e *payment.PaymentEvent) error
}

type service struct {
	repo payment.Repository
}

func NewService(repo payment.Repository) Service {
	return &service{
		repo: repo,
	}
}

func (s *service) CreatePayment(ctx context.Context, p *payment.Payment) error {
	// Simple idempotency check
	if p.IdempotencyKey != "" {
		existing, err := s.repo.GetByIdempotencyKey(ctx, p.IdempotencyKey)
		if err == nil && existing != nil {
			*p = *existing // Return existing payment
			return nil
		}
	}

	// Use domain factory to initialize domain-specific fields
	newPayment, err := payment.NewPayment(
		id.GeneratePaymentID(),
		id.GenerateTransactionID(),
		p.CustomerID.Value(),
		p.Amount.Value(),
		string(p.Currency), // NewPayment expects string, not Currency type
		p.Description,
		p.IdempotencyKey,
	)
	if err != nil {
		return err // Propagate validation error from domain
	}

	// Update the original pointer with initial domain state
	*p = *newPayment

	return s.repo.Save(ctx, p)
}

func (s *service) GetPayment(ctx context.Context, paymentID string) (*payment.Payment, error) {
	return s.repo.GetByID(ctx, paymentID)
}

func (s *service) ProcessEvent(ctx context.Context, e *payment.PaymentEvent) error {
	p, err := s.repo.GetByID(ctx, e.PaymentID)
	if err != nil {
		return err
	}

	var nextStatus payment.Status
	switch e.Type {
	case payment.EventPaymentCompleted:
		nextStatus = payment.StatusCompleted
	case payment.EventPaymentFailed:
		nextStatus = payment.StatusFailed
	case payment.EventPaymentCancelled:
		nextStatus = payment.StatusCancelled
	default:
		return fmt.Errorf("unknown event type: %s", e.Type)
	}

	// Idempotency: if the payment has already reached the status reported by the event, stop here.
	if p.Status == nextStatus {
		return nil
	}

	// Domain rules require Pending -> Processing before reaching terminal states (Completed, Failed, or Cancelled).
	if p.Status == payment.StatusPending {
		if err := p.Transition(payment.StatusProcessing); err != nil {
			return err
		}
	}

	if err := p.Transition(nextStatus); err != nil {
		return err
	}

	// Only update transaction ID if provided in the event
	if e.TransactionID != "" {
		p.TransactionID = e.TransactionID
	}
	return s.repo.Save(ctx, p)
}
