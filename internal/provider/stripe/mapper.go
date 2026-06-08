package stripe

import (
	"strings"
)

// MapStripeStatus maps raw Stripe PaymentIntent status string to standard internal states:
// e.g., succeeded -> completed, processing -> processing, failed -> failed, canceled -> cancelled, etc.
func MapStripeStatus(stripeStatus string) string {
	switch strings.ToLower(stripeStatus) {
	case "succeeded":
		return "completed"
	case "requires_payment_method", "requires_confirmation", "requires_action", "processing", "requires_capture":
		return "processing"
	case "canceled":
		return "cancelled"
	case "failed":
		return "failed"
	default:
		return "pending"
	}
}

// MapStripeEventToStandard maps Stripe's webhook event type to our generalized internal event type.
func MapStripeEventToStandard(stripeEventType string) string {
	switch stripeEventType {
	case "payment_intent.succeeded", "charge.succeeded":
		return "payment.completed"
	case "payment_intent.payment_failed", "charge.failed":
		return "payment.failed"
	case "payment_intent.canceled":
		return "payment.cancelled"
	default:
		return stripeEventType
	}
}
