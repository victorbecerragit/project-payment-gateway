# Stripe Payment Provider Adapter — Technical Delivery Report

This report serves as the permanent documentation for the standard library-based Stripe payment provider adapter implementation. It details the file structure, contract integration, state/event mapping strategies, signature validation algorithms, and testing matrices.

---

## 1. Directory Structure and Architectural Placement

The adapter isolates all Stripe-specific requirements, third-party error mapping patterns, and custom serialization formats within its own directory tree:

- [internal/provider/stripe/client.go](internal/provider/stripe/client.go) — Houses initialization options, client timeouts, custom url-encoding payload helpers, and the outbound raw `http.Client` operations.
- [internal/provider/stripe/mapper.go](internal/provider/stripe/mapper.go) — Handles state-to-state translations, converting raw Stripe response statuses to gateway primitives.
- [internal/provider/stripe/webhook.go](internal/provider/stripe/webhook.go) — Houses Stripe event parsing, metadata/id identifier lookups, and timing-safe webhook HMAC verification.
- [internal/provider/stripe/stripe_test.go](internal/provider/stripe/stripe_test.go) — Multi-case isolated unit tests mocking external endpoints using Go standard library utilities.

---

## 2. Abstraction Framework & Interface Implementations

The gateway interacts with the Stripe adapter solely through our generic, provider-agnostic core contracts defined in [internal/provider/provider.go](internal/provider/provider.go).

### The Provider Interface
`StripeProvider` implements the contract:

```go
type Provider interface {
	CreatePayment(ctx context.Context, req *CreatePaymentRequest) (*CreatePaymentResponse, error)
	ParseWebhook(ctx context.Context, payload []byte, signature string) (*WebhookEvent, error)
	Name() string
}
```

The adapter maps structures without leaking vendor-specific imports outside its borders:
- Converts float64 monetary amounts to integer-based cents (`Amount * 100`) as expected by modern ledger entities.
- Binds standard tracking IDs (`payment_id`, `customer_id`, `idempotency_key`) inside the Stripe `metadata` container.
- Embeds API authentication credentials into standard outbound `Authorization: Bearer <sk_key>` headers.

---

## 3. Webhook Signature Verification and Timing Attack Defense

Stripe authenticates notify requests using signatures. [internal/provider/stripe/webhook.go](internal/provider/stripe/webhook.go) implements signature verification matching Stripe's specification:

1. **Header Parsing**: Iterates over parts of the `X-Webhook-Signature` string extracting:
   - Timestamp `t`
   - All signature instances of `v1`
2. **Payload Computation**: Signs the concatenated payload using HMAC-SHA256:
   - `signed_payload = timestamp + "." + request_body`
3. **Timing-Safe Evaluation**: Utilizes standard library `crypto/subtle` equivalents via `hmac.Equal` to prevent malicious actors from inferring active keys via side-channel analysis.

---

## 4. Normalization and Mapping Strategy

Status and event translations preserve stable generic endpoints for downstream clients:

### Payment Intent Status Mapping

| Stripe Status | Gateway Status | Context |
| :--- | :--- | :--- |
| `succeeded` | `completed` | Transaction finished successfully. |
| `requires_payment_method` | `processing` | Form collected, awaiting client authentication. |
| `requires_confirmation` | `processing` | Card captured, waiting confirmation step. |
| `requires_action` | `processing` | Client-facing active 3D-Secure prompts. |
| `processing` | `processing` | Asynchronous settlement path in progress. |
| `requires_capture` | `processing` | Held on card, authorizing before Capture. |
| `canceled` | `cancelled` | Payment intent cancelled by operator/customer. |
| `failed` | `failed` | Card declined or transaction rejected. |

### Webhook Event Mapping

| Stripe Event Type | Gateway Event Type | Event Code |
| :--- | :--- | :--- |
| `payment_intent.succeeded` | `payment.completed` | EventPaymentCompleted |
| `charge.succeeded` | `payment.completed` | EventPaymentCompleted |
| `payment_intent.payment_failed` | `payment.failed` | EventPaymentFailed |
| `charge.failed` | `payment.failed` | EventPaymentFailed |
| `payment_intent.canceled` | `payment.cancelled` | EventPaymentCancelled |

---

## 5. Execution and Verification Commands

The code has zero external dependencies, making build execution robust and reliable.

- **Check clean compilation**:
  ```bash
  go build ./...
  ```
- **Execute unit tests**:
  ```bash
  go test -v ./internal/provider/stripe/...
  ```
- **Execute the entire suite**:
  ```bash
  go test -v ./...
  ```
