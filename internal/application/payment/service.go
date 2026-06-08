package apppayment

import (
	"context"
	"fmt"
	"time"

	"github.com/victorbecerragit/project-payment-gateway/internal/domain/payment"
	"github.com/victorbecerragit/project-payment-gateway/internal/platform/id"
	"github.com/victorbecerragit/project-payment-gateway/internal/provider"
)

type Service interface {
	CreatePayment(ctx context.Context, p *payment.Payment) error
	GetPayment(ctx context.Context, paymentID string) (*payment.Payment, error)
	ProcessEvent(ctx context.Context, e *payment.PaymentEvent) error
	ParseWebhook(ctx context.Context, payload []byte, signature string) (*payment.PaymentEvent, error)
}

type service struct {
	repo     payment.Repository
	provider provider.Provider
}

func NewService(repo payment.Repository, prov provider.Provider) Service {
	return &service{
		repo:     repo,
		provider: prov,
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

	// Call provider to create payment with provider (e.g., Stripe, PayPal)
	providerReq := &provider.CreatePaymentRequest{
		PaymentID:      p.ID,
		Amount:         int64(p.Amount.Value() * 100), // Convert to cents
		Currency:       string(p.Currency),
		Description:    p.Description,
		CustomerID:     p.CustomerID.Value(),
		IdempotencyKey: p.IdempotencyKey,
	}

	providerResp, err := s.provider.CreatePayment(ctx, providerReq)
	if err != nil {
		return fmt.Errorf("provider %s failed to create payment: %w", s.provider.Name(), err)
	}

	// Update payment with provider transaction ID
	p.TransactionID = providerResp.TransactionID

	return s.repo.Save(ctx, p)
}

func (s *service) GetPayment(ctx context.Context, paymentID string) (*payment.Payment, error) {
	return s.repo.GetByID(ctx, paymentID)
}

func (s *service) ProcessEvent(ctx context.Context, e *payment.PaymentEvent) error {
	p, err := s.repo.GetByID(ctx, e.PaymentID)
	if err != nil && e.TransactionID != "" {
		// Fallback: provider webhook may not carry metadata.payment_id
		// (e.g. Stripe Dashboard retry or metadata stripped by a provider update).
		// Attempt lookup by provider reference stored in TransactionID.
		p, err = s.repo.GetByProviderRef(ctx, e.TransactionID)
	}
	if err != nil {
		return fmt.Errorf("payment not found by id %q or provider ref %q: %w", e.PaymentID, e.TransactionID, err)
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

// ParseWebhook translates a provider-specific webhook payload into a domain PaymentEvent.
func (s *service) ParseWebhook(ctx context.Context, payload []byte, signature string) (*payment.PaymentEvent, error) {
	// Delegate to provider to parse webhook
	webhookEvent, err := s.provider.ParseWebhook(ctx, payload, signature)
	if err != nil {
		return nil, fmt.Errorf("failed to parse webhook: %w", err)
	}

	// Translate provider event type to domain event type.
	// Unrecognized event types are rejected explicitly — silently defaulting to
	// "completed" risks incorrectly advancing the payment state machine.
	var domainEventType payment.EventType
	switch webhookEvent.EventType {
	case "payment.completed":
		domainEventType = payment.EventPaymentCompleted
	case "payment.failed":
		domainEventType = payment.EventPaymentFailed
	case "payment.cancelled":
		domainEventType = payment.EventPaymentCancelled
	default:
		return nil, fmt.Errorf("unrecognized provider event type %q: no domain mapping defined", webhookEvent.EventType)
	}

	// Return domain PaymentEvent
	return &payment.PaymentEvent{
		Type:          domainEventType,
		PaymentID:     webhookEvent.PaymentID,
		TransactionID: webhookEvent.TransactionID,
		Timestamp:     time.Now(),
	}, nil
}
