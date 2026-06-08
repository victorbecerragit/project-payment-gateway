package apppayment

import (
	"context"
	"fmt"
	"time"

	"github.com/victorbecerra/kube-refresh/project-payment-gateway/internal/platform/slogext"
	"github.com/victorbecerra/kube-refresh/project-payment-gateway/internal/platform/tracing"
	"github.com/victorbecerragit/project-payment-gateway/internal/domain/payment"
	"github.com/victorbecerragit/project-payment-gateway/internal/platform/id"
	"github.com/victorbecerragit/project-payment-gateway/internal/provider"
)

type Service interface {
	CreatePayment(ctx context.Context, p *payment.Payment) error
	GetPayment(ctx context.Context, paymentID string) (*payment.Payment, error)
	ProcessEvent(ctx context.Context, e *payment.PaymentEvent) error
	ParseWebhook(ctx context.Context, payload []byte, signature string) (*payment.PaymentEvent, error)
	// No SetLogger needed, use slogext.Ctx(ctx)
}

type service struct {
	repo     payment.Repository
	provider provider.Provider
	tracer   tracing.Tracer
}

func NewService(repo payment.Repository, prov provider.Provider, tracer tracing.Tracer) Service {
	if tracer == nil {
		tracer = tracing.NewNoOpTracer()
	}
	return &service{
		repo:     repo,
		provider: prov,
		tracer:   tracer,
	}
}

func (s *service) CreatePayment(ctx context.Context, p *payment.Payment) error {
	// Simple idempotency check
	if p.IdempotencyKey != "" {
		existing, err := s.repo.GetByIdempotencyKey(ctx, p.IdempotencyKey)
		if err != nil {
			slogext.Ctx(ctx).Error("failed to check idempotency key", "error", err)
		}
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

	ctx, span := s.tracer.StartSpan(ctx, "provider.CreatePayment")
	defer span.End()

	providerResp, err := s.provider.CreatePayment(ctx, providerReq)
	if err != nil {
		return fmt.Errorf("provider %s failed to create payment: %w", s.provider.Name(), err)
	}

	// Update payment with provider transaction ID
	p.TransactionID = providerResp.TransactionID

	span.SetAttribute("payment.id", p.ID)
	span.SetAttribute("provider.transaction_id", p.TransactionID)

	return s.repo.Save(ctx, p)
}

func (s *service) GetPayment(ctx context.Context, paymentID string) (*payment.Payment, error) {
	ctx, span := s.tracer.StartSpan(ctx, "repo.GetByID")
	defer span.End()
	span.SetAttribute("payment.id", paymentID)
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
		slogext.Ctx(ctx).Error("payment not found by id or provider ref", "payment_id", e.PaymentID, "provider_ref", e.TransactionID, "error", err)
	}
	if err != nil {
		return fmt.Errorf("payment not found by id %q or provider ref %q: %w", e.PaymentID, e.TransactionID, err)
	}

	ctx, span := s.tracer.StartSpan(ctx, "app.ProcessEvent")
	defer span.End()
	span.SetAttribute("payment.id", p.ID)
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
		slogext.Ctx(ctx).Warn("unknown event type", "event_type", e.Type)
	}

	// Idempotency: if the payment has already reached the status reported by the event, stop here.
	if p.Status == nextStatus {
		return nil
	}

	// Domain rules require Pending -> Processing before reaching terminal states (Completed, Failed, or Cancelled).
	if p.Status == payment.StatusPending {
		if err := p.Transition(payment.StatusProcessing); err != nil {
			slogext.Ctx(ctx).Error("failed to transition payment to processing", "payment_id", p.ID, "error", err)
			return err
		}
	}

	if err := p.Transition(nextStatus); err != nil {
		slogext.Ctx(ctx).Error("failed to transition payment status", "payment_id", p.ID, "from", p.Status, "to", nextStatus, "error", err)
		return err
	}

	// Only update transaction ID if provided in the event
	if e.TransactionID != "" {
		p.TransactionID = e.TransactionID
	}

	span.SetAttribute("payment.status", p.Status)
	span.SetAttribute("event.type", e.Type)

	return s.repo.Save(ctx, p)
}

// ParseWebhook translates a provider-specific webhook payload into a domain PaymentEvent.
func (s *service) ParseWebhook(ctx context.Context, payload []byte, signature string) (*payment.PaymentEvent, error) {
	// Delegate to provider to parse webhook
	webhookEvent, err := s.provider.ParseWebhook(ctx, payload, signature)
	if err != nil {
		slogext.Ctx(ctx).Error("failed to parse webhook from provider", "error", err)
	}
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
		slogext.Ctx(ctx).Warn("unrecognized provider event type", "provider_event_type", webhookEvent.EventType)
	}

	// Return domain PaymentEvent
	return &payment.PaymentEvent{
		Type:          domainEventType,
		PaymentID:     webhookEvent.PaymentID,
		TransactionID: webhookEvent.TransactionID,
		Timestamp:     time.Now(),
	}, nil
}
