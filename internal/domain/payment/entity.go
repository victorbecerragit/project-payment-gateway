package payment

import (
	"fmt"
	"time"
)

// Status represents the state of a payment
type Status string

const (
	StatusPending    Status = "pending"
	StatusProcessing Status = "processing"
	StatusCompleted  Status = "completed"
	StatusFailed     Status = "failed"
	StatusCancelled  Status = "cancelled"
)

// Payment is the core domain entity
type Payment struct {
	ID             string
	Amount         float64
	Currency       string
	Status         Status
	IdempotencyKey string
	TransactionID  string
	CustomerID     string
	Description    string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// EventType represents a domain event type
type EventType string

const (
	EventPaymentCompleted EventType = "payment.completed"
	EventPaymentFailed    EventType = "payment.failed"
)

// PaymentEvent is a domain event
type PaymentEvent struct {
	Type          EventType
	PaymentID     string
	TransactionID string
	Timestamp     time.Time
}

// CanTransitionTo checks if a status transition is valid
func (p *Payment) CanTransitionTo(next Status) bool {
	switch p.Status {
	case StatusPending:
		return next == StatusProcessing || next == StatusCompleted || next == StatusFailed || next == StatusCancelled
	case StatusProcessing:
		return next == StatusCompleted || next == StatusFailed || next == StatusCancelled
	case StatusCompleted, StatusFailed, StatusCancelled:
		return false
	default:
		return false
	}
}

// Transition updates the payment status if the transition is valid
func (p *Payment) Transition(next Status) error {
	if !p.CanTransitionTo(next) {
		return fmt.Errorf("invalid status transition from %s to %s", p.Status, next)
	}
	p.Status = next
	p.UpdatedAt = time.Now()
	return nil
}

// NewPayment creates a new payment aggregate with initial status and timestamps
func NewPayment(id, txID, customerID string, amount float64, currency, description, idempotencyKey string) *Payment {
	now := time.Now()
	return &Payment{
		ID:             id,
		TransactionID:  txID,
		CustomerID:     customerID,
		Amount:         amount,
		Currency:       currency,
		Description:    description,
		IdempotencyKey: idempotencyKey,
		Status:         StatusPending,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}
