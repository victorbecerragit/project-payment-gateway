package payment

import (
	"fmt"
	"strings"
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

// Currency represents an ISO 4217 currency code
type Currency string

// globalSupportedCurrencies will be populated from config
var globalSupportedCurrencies map[Currency]bool

// SetSupportedCurrencies initializes the global list of supported currencies.
// This should be called once at application startup.
func SetSupportedCurrencies(currencies []string) {
	globalSupportedCurrencies = make(map[Currency]bool)
	for _, c := range currencies {
		globalSupportedCurrencies[Currency(c)] = true
	}
}

// GetSupportedCurrencies returns a copy of the currently supported currencies for testing.
func GetSupportedCurrencies() map[Currency]bool {
	result := make(map[Currency]bool)
	for k, v := range globalSupportedCurrencies {
		result[k] = v
	}
	return result
}

// IsValid checks if the currency is supported by the gateway
func (c Currency) IsValid() bool {
	return globalSupportedCurrencies[c]
}

// CustomerID is a value object representing a validated customer identifier
type CustomerID struct {
	value string
}

// NewCustomerID creates a new CustomerID value object and validates its format
func NewCustomerID(v string) (CustomerID, error) {
	if !strings.HasPrefix(v, "cust_") || len(v) <= len("cust_") {
		return CustomerID{}, &PaymentError{
			Type:    ErrorTypeDomain,
			Message: "customer ID must have 'cust_' prefix and not be empty",
		}
	}
	return CustomerID{value: v}, nil
}

// MustNewCustomerID is a helper for tests and mappers where valid input is expected
func MustNewCustomerID(v string) CustomerID {
	cid, err := NewCustomerID(v)
	if err != nil {
		panic(err)
	}
	return cid
}

// Value returns the raw string value of the CustomerID
func (c CustomerID) Value() string {
	return c.value
}

// Amount is a value object representing a payment amount
type Amount struct {
	value float64
}

// NewAmount creates a new Amount value object and validates it
func NewAmount(v float64) (Amount, error) {
	if v <= 0 {
		return Amount{}, &PaymentError{
			Type:    ErrorTypeDomain,
			Message: "payment amount must be positive",
		}
	}
	return Amount{value: v}, nil
}

// MustNewAmount is a helper for tests and mappers where valid input is expected
func MustNewAmount(v float64) Amount {
	a, err := NewAmount(v)
	if err != nil {
		panic(err)
	}
	return a
}

// Value returns the raw float64 value
func (a Amount) Value() float64 {
	return a.value
}

// ErrorType defines categories for payment-related errors
type ErrorType string

const (
	ErrorTypeStateMachine ErrorType = "state_machine_violation"
	ErrorTypeDomain       ErrorType = "domain_logic"
)

// PaymentError is a custom error type for domain violations
type PaymentError struct {
	Type    ErrorType
	Message string
}

func (e *PaymentError) Error() string {
	return e.Message
}

// Payment is the core domain entity
type Payment struct {
	ID             string
	Amount         Amount
	Currency       Currency
	Status         Status
	IdempotencyKey string
	TransactionID  string
	CustomerID     CustomerID
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
		return &PaymentError{
			Type:    ErrorTypeStateMachine,
			Message: fmt.Sprintf("invalid status transition from %s to %s", p.Status, next),
		}
	}
	p.Status = next
	p.UpdatedAt = time.Now().UTC()
	return nil
}

// Cancel moves the payment to the cancelled status
func (p *Payment) Cancel() error {
	return p.Transition(StatusCancelled)
}

// NewPayment creates a new payment with initial state
func NewPayment(id, transactionID, customerID string, amount float64, currency, description, idempotencyKey string) (*Payment, error) {
	amt, err := NewAmount(amount)
	if err != nil {
		return nil, err
	}
	curr := Currency(strings.ToUpper(currency))
	if !curr.IsValid() {
		return nil, &PaymentError{
			Type:    ErrorTypeDomain,
			Message: fmt.Sprintf("unsupported or invalid currency: %s", currency),
		}
	}

	custID, err := NewCustomerID(customerID)
	if err != nil {
		return nil, err
	}

	if description == "" {
		return nil, &PaymentError{
			Type:    ErrorTypeDomain,
			Message: "description is required",
		}
	}

	return &Payment{
		ID:             id,
		TransactionID:  transactionID,
		CustomerID:     custID,
		Amount:         amt,
		Currency:       curr,
		Description:    description,
		Status:         StatusPending,
		IdempotencyKey: idempotencyKey,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}, nil
}
