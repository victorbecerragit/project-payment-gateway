package models

import "time"

// Payment statuses
const (
	StatusPending    = "pending"
	StatusProcessing = "processing"
	StatusCompleted  = "completed"
	StatusFailed     = "failed"
	StatusCancelled  = "cancelled"
)

// PaymentRequest represents a payment initiation request
type PaymentRequest struct {
	Amount      float64 `json:"amount"`
	Currency    string  `json:"currency"`
	Description string  `json:"description"`
	CustomerID  string  `json:"customer_id"`
	CardToken   string  `json:"card_token,omitempty"`
}

// PaymentResponse represents the response after initiating a payment
type PaymentResponse struct {
	PaymentID     string    `json:"payment_id"`
	Status        string    `json:"status"`
	Amount        float64   `json:"amount"`
	Currency      string    `json:"currency"`
	TransactionID string    `json:"transaction_id,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// PaymentStatus represents the current status of a payment
type PaymentStatus struct {
	PaymentID     string    `json:"payment_id"`
	Status        string    `json:"status"`
	TransactionID string    `json:"transaction_id,omitempty"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// WebhookPayload represents webhook notification from payment provider
type WebhookPayload struct {
	EventType     string    `json:"event_type"`
	PaymentID     string    `json:"payment_id"`
	Status        string    `json:"status"`
	TransactionID string    `json:"transaction_id"`
	Timestamp     time.Time `json:"timestamp"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Code    int    `json:"code"`
}

// Payment represents a payment record
type Payment struct {
	ID             string    `json:"id"`
	Amount         float64   `json:"amount"`
	Currency       string    `json:"currency"`
	Status         string    `json:"status"`
	IdempotencyKey string    `json:"idempotency_key"`
	TransactionID  string    `json:"transaction_id,omitempty"`
	CustomerID     string    `json:"customer_id"`
	Description    string    `json:"description,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
