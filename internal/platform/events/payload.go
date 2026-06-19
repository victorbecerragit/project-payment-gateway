package events

import "time"

// PaymentEvent is the canonical event published to the payment-events Kafka topic.
// All fields are required. Do not add optional fields without updating the consumer.
type PaymentEvent struct {
	EventID   string    `json:"event_id"`
	EventType string    `json:"event_type"`
	PaymentID string    `json:"payment_id"`
	Provider  string    `json:"provider"`
	Amount    float64   `json:"amount"`
	Currency  string    `json:"currency"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}
