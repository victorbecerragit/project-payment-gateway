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

// validTransitions defines all legal state transitions.
// pending can only move to processing or cancelled (must process before completing/failing).
// processing can move to completed, failed, or cancelled.
// Terminal states (completed, failed, cancelled) accept no further transitions.
var validTransitions = map[Status][]Status{
	StatusPending:    {StatusProcessing, StatusCancelled},
	StatusProcessing: {StatusCompleted, StatusFailed, StatusCancelled},
}

// CanTransitionTo returns true when transitioning from the current status to next is allowed.
func (p *Payment) CanTransitionTo(next Status) bool {
	allowed, ok := validTransitions[p.Status]
	if !ok {
		return false // terminal state
	}
	for _, s := range allowed {
		if s == next {
			return true
		}
	}
	return false
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

// NewPayment creates a new payment with initial state
func NewPayment(id, transactionID, customerID string, amount float64, currency, description, idempotencyKey string) (*Payment, error) {
	if amount <= 0 {
		return nil, fmt.Errorf("payment amount must be positive")
	}
	if currency == "" {
		return nil, fmt.Errorf("currency is required")
	}

	return &Payment{
		ID:             id,
		TransactionID:  transactionID,
		CustomerID:     customerID,
		Amount:         amount,
		Currency:       currency,
		Description:    description,
		Status:         StatusPending,
		IdempotencyKey: idempotencyKey,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}, nil
}
