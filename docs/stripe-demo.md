# Stripe End-to-End Demo Guide

This document outlines how to test the Payment Gateway end-to-end using Stripe as a real Payment Service Provider (PSP) in a sandbox environment.

## Overview

The demonstration uses the gateway's generic API and underlying Stripe provider logic to transition a payment through its lifecycle securely and idempotently via webhook events.

**Expected Flow**:
1. **Create Payment**: Client calls `POST /api/v1/payments`. The gateway securely creates a Stripe `PaymentIntent` and returns a gateway Payment ID along with the pending status.
2. **Provider Action**: The client completes the payment interaction using the Stripe frontend elements or a synthetic event is triggered via Stripe CLI.
3. **Webhook Callback**: Stripe sends a webhook to `POST /api/v1/webhooks/payment`.
4. **State Transition**: The gateway verifies the webhook signature, maps the Stripe event to a domain event, and idempotently updates the payment status (e.g., to `completed` or `failed`).
5. **Status Verification**: Client calls `GET /api/v1/payments/{payment_id}` to verify completion.

## Local Configuration (Docker Compose)

1. Follow the local setup steps in `extras/stripe-sandbox/README.md` to configure the root `.env` file (copied from the sandbox example). Use a **test mode secret key** (`sk_test_...`) from the [Stripe Dashboard](https://dashboard.stripe.com/test/apikeys).

2. Unset any stale shell env vars, then start the stack:
   ```bash
   unset STRIPE_API_KEY STRIPE_WEBHOOK_SECRET
   docker compose up --build -d
   ```

3. In a separate terminal, start the Stripe CLI forwarder:
   ```bash
   stripe listen --forward-to localhost:8080/api/v1/webhooks/payment
   ```
   Copy the `whsec_test_...` secret printed by `stripe listen`, update `STRIPE_WEBHOOK_SECRET` in `.env`, then restart the app:
   ```bash
   docker compose up -d --no-deps payment-gateway
   ```

4. Create a payment and note the `payment_id` in the response:
   ```bash
   curl -s -X POST http://localhost:8080/api/v1/payments \
     -H "Content-Type: application/json" \
     -H "X-Idempotency-Key: test-$(date +%s)" \
     -d '{"amount":99.99,"currency":"USD","description":"test","customer_id":"cust_123"}'
   ```

5. Trigger a webhook for that specific payment:
   ```bash
   stripe trigger payment_intent.succeeded \
     --override "payment_intent:metadata[payment_id]=<payment_id>"
   ```

6. Verify the payment status updated to `completed`:
   ```bash
   docker compose logs payment-gateway | grep -E "webhook|error|warn"
   docker compose exec db psql -U user -d payment_gateway \
     -c "SELECT id, status, updated_at FROM payments ORDER BY created_at DESC LIMIT 5;"
   ```

## Kubernetes Flow

When running in Kubernetes, ensure the webhook endpoint is either publicly reachable (via Ingress) to receive hits directly from the Stripe backend or use the Stripe CLI forwarded to a pod port (using `kubectl port-forward`).

## Testing the Happy Path

1. **Initiate Payment**:
   ```bash
   IDEM="demo-$(date +%s)"
   PAYMENT=$(curl -s -X POST http://localhost:8080/api/v1/payments \
     -H "Content-Type: application/json" \
     -H "X-Idempotency-Key: $IDEM" \
     -d '{"amount":99.99,"currency":"USD","description":"Stripe demo","customer_id":"cust_123"}')
   echo $PAYMENT
   PAY_ID=$(echo $PAYMENT | python3 -c "import sys,json; print(json.load(sys.stdin)['payment_id'])")
   ```

2. **Simulate Success**:
   ```bash
   stripe trigger payment_intent.succeeded \
     --override "payment_intent:metadata[payment_id]=$PAY_ID"
   ```

3. **Verify Status**:
   ```bash
   curl -s http://localhost:8080/api/v1/payments/$PAY_ID
   ```
   Expected: `"status": "completed"`

## Testing Failure Modes

- **Simulate Payment Failure**:
  ```bash
  stripe trigger payment_intent.payment_failed \
    --override "payment_intent:metadata[payment_id]=$PAY_ID"
  ```
  Expected: `"status": "failed"`

## Troubleshooting

### Invalid Webhook Signature
**Symptom**: Webhook requests are rejected with `400` (bad signature) or `401` (missing header).
**Fix**: Ensure `STRIPE_WEBHOOK_SECRET` exactly matches the secret from `stripe listen` and restart the app after updating `.env`.

### Missing Metadata or Referencing Issues
**Symptom**: The gateway receives the webhook but returns `404 payment not found`.
**Fix**: Always pass `--override "payment_intent:metadata[payment_id]=<pay_id>"` when triggering events via Stripe CLI. The `pay_id` must be an ID previously returned by `POST /api/v1/payments`.

### Idempotency Mismatches
**Symptom**: Replayed webhooks cause errors or unexpected state changes.
**Fix**: Webhook processing is fully idempotent — replays return `200` and are silently skipped. Check that `X-Idempotency-Key` is sent on payment creation requests.

### Unknown Event Type
**Symptom**: The webhook is acknowledged (`200 OK`) but the payment state isn't updated.
**Fix**: Check application logs for `ignoring unrecognized provider event type`. The gateway only processes `payment_intent.succeeded`, `payment_intent.payment_failed`, and `payment_intent.canceled`.
