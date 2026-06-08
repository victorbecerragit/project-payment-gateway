package provider

import "context"

// Provider defines the interface for payment provider adapters.
// Implementations handle provider-specific payment creation, webhook parsing,
// and other PSP-specific operations.
type Provider interface {
	// CreatePayment submits a payment request to the provider and returns the provider response.
	// The caller is responsible for translating domain Payment and provider response.
	CreatePayment(ctx context.Context, req *CreatePaymentRequest) (*CreatePaymentResponse, error)

	// ParseWebhook decodes and validates a provider webhook payload.
	// Provider-specific signature verification should happen at this layer.
	ParseWebhook(ctx context.Context, payload []byte, signature string) (*WebhookEvent, error)

	// Name returns a unique identifier for this provider (e.g., "stripe", "paypal", "mock")
	Name() string
}

// CreatePaymentRequest encapsulates provider-specific payment creation parameters.
type CreatePaymentRequest struct {
	// PaymentID from the domain
	PaymentID string

	// Amount in the smallest currency unit (e.g., cents for USD)
	Amount int64

	// Currency ISO 4217 code (e.g., "USD", "EUR")
	Currency string

	// Description of the payment
	Description string

	// CustomerID from the domain
	CustomerID string

	// Provider-specific payload; marshaled from the HTTP request
	// e.g., StripeToken for Stripe, PayPalEmail for PayPal
	PaymentMethod interface{}

	// IdempotencyKey for provider idempotency
	IdempotencyKey string
}

// CreatePaymentResponse encapsulates the provider's response to a payment creation request.
type CreatePaymentResponse struct {
	// TransactionID assigned by the provider (e.g., Stripe charge ID)
	TransactionID string

	// Status of the payment at the provider (e.g., "succeeded", "processing")
	ProviderStatus string

	// ProviderData holds provider-specific metadata (e.g., Stripe ChargeID, PayPal OrderID)
	// Can be serialized for storage in the Payment entity.
	ProviderData map[string]interface{}
}

// WebhookEvent represents a normalized webhook event from any provider.
// The provider adapter is responsible for translating provider-specific webhooks
// into this unified interface.
type WebhookEvent struct {
	// PaymentID from the webhook (payment ID stored in our system, not provider's)
	PaymentID string

	// TransactionID from the webhook (provider's transaction ID)
	TransactionID string

	// EventType indicates what happened (e.g., "payment.completed", "payment.failed")
	EventType string

	// ProviderEventType is the raw provider event type for logging/debugging
	ProviderEventType string

	// Status represents the payment status according to the provider
	Status string

	// ProviderData contains provider-specific webhook fields
	ProviderData map[string]interface{}
}

// ErrProviderError is a sentinel error type for provider-level failures
type ErrProviderError struct {
	Provider string
	Message  string
	Code     string // Provider error code if available
}

func (e *ErrProviderError) Error() string {
	return e.Message
}
