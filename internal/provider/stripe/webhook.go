package stripe

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/victorbecerragit/project-payment-gateway/internal/provider"
)

// VerifySignature cryptographically validates Stripe webhook payloads using the webhook secret signing key,
// matching the Stripe t=<timestamp>,v1=<sig> header specification.
func VerifySignature(payload []byte, signatureHeader string, secret string) error {
	if signatureHeader == "" {
		return fmt.Errorf("empty signature header")
	}

	var timestampStr string
	var signatures []string

	parts := strings.Split(signatureHeader, ",")
	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		val := strings.TrimSpace(kv[1])

		if key == "t" {
			timestampStr = val
		} else if key == "v1" {
			signatures = append(signatures, val)
		}
	}

	if timestampStr == "" {
		return fmt.Errorf("missing timestamp 't' in signature")
	}
	if len(signatures) == 0 {
		return fmt.Errorf("missing signature 'v1' in signature")
	}

	// Double check timestamp integer validation to guarantee format safety
	if _, err := strconv.ParseInt(timestampStr, 10, 64); err != nil {
		return fmt.Errorf("invalid timestamp format: %w", err)
	}

	// Signed payload is `<timestamp>.<payload>`
	signedPayload := []byte(timestampStr + "." + string(payload))

	// Compute HMAC-SHA256 signature
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(signedPayload)
	expectedMAC := mac.Sum(nil)

	// Securely compare with all received signatures to defend against timing attacks
	for _, sig := range signatures {
		sigBytes, err := hex.DecodeString(sig)
		if err != nil {
			continue
		}
		if hmac.Equal(sigBytes, expectedMAC) {
			return nil // A signature match was found!
		}
	}

	return fmt.Errorf("signature mismatch")
}

// ParseWebhook decodes the Stripe-specific event payload, verifies the signature if WebhookSecret is set,
// maps raw statuses to standardized ones, and extracts tracking identifiers for generic routing.
func (p *StripeProvider) ParseWebhook(ctx context.Context, payload []byte, signature string) (*provider.WebhookEvent, error) {
	if len(payload) == 0 {
		return nil, &provider.ErrProviderError{
			Provider: p.Name(),
			Message:  "payload is empty",
			Code:     "invalid_webhook_payload",
		}
	}

	// Optional signature verification check
	if p.config.WebhookSecret != "" {
		if err := VerifySignature(payload, signature, p.config.WebhookSecret); err != nil {
			return nil, &provider.ErrProviderError{
				Provider: p.Name(),
				Message:  fmt.Sprintf("webhook signature verification failed: %v", err),
				Code:     "invalid_webhook_signature",
			}
		}
	}

	var stripeEvent struct {
		ID   string `json:"id"`
		Type string `json:"type"`
		Data struct {
			Object struct {
				ID       string                 `json:"id"`
				Object   string                 `json:"object"`
				Status   string                 `json:"status"`
				Metadata map[string]interface{} `json:"metadata"`
			} `json:"object"`
		} `json:"data"`
	}

	if err := json.Unmarshal(payload, &stripeEvent); err != nil {
		return nil, &provider.ErrProviderError{
			Provider: p.Name(),
			Message:  fmt.Sprintf("failed to parse stripe event payload: %v", err),
			Code:     "malformed_json",
		}
	}

	// Extract stored internal payment identifiers
	paymentID, _ := stripeEvent.Data.Object.Metadata["payment_id"].(string)
	if paymentID == "" {
		// Fallback to idempotency key as secondary source if payment_id is absent
		paymentID, _ = stripeEvent.Data.Object.Metadata["idempotency_key"].(string)
	}

	standardEventType := MapStripeEventToStandard(stripeEvent.Type)
	standardPaymentStatus := MapStripeStatus(stripeEvent.Data.Object.Status)

	providerData := map[string]interface{}{
		"stripe_event_id":       stripeEvent.ID,
		"stripe_event_type":     stripeEvent.Type,
		"stripe_payment_status": stripeEvent.Data.Object.Status,
	}

	return &provider.WebhookEvent{
		PaymentID:         paymentID,
		TransactionID:     stripeEvent.Data.Object.ID,
		EventType:         standardEventType,
		ProviderEventType: stripeEvent.Type,
		Status:            standardPaymentStatus,
		ProviderData:      providerData,
	}, nil
}
