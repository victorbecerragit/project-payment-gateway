# Stripe End-to-End Demo Guide

This document outlines how to test the Payment Gateway end-to-end using Stripe as a real Payment Service Provider (PSP) in a sandbox environment.

## Overview

The demonstration uses the gateway's generic API and underlying Stripe provider logic to transition a payment through its lifecycle securely and idempotently via webhook events.

**Expected Flow**:
1. **Create Payment**: Client calls `POST /api/v1/payments`. The gateway securely creates a Stripe `PaymentIntent` and returns a gateway Payment ID along with the pending status.
2. **Provider Action**: The client completes the payment interaction using the Stripe frontend elements or a synthetic event is triggered via Stripe CLI.
3. **Webhook Callback**: Stripe sends a webhook to `POST /api/v1/webhooks/payment`.
4. **State Transition**: The gateway verifies the webhook signature, maps the Stripe event to a domain event, and idempotently updates the payment status (e.g., to `COMPLETED` or `FAILED`).
5. **Status Verification**: Client calls `GET /api/v1/payments/status` to verify completion.

## Local Configuration (Docker Compose)

1. Follow the local setup steps in `extras/stripe-sandbox/README.md` to configure the root `.env` file (copied from the sandbox example).
2. Start the payment gateway using `docker-compose up --build`.
3. In a separate terminal, run the Stripe CLI forwarder:
   ```bash
   stripe listen --forward-to localhost:8080/api/v1/webhooks/payment
   ```

## Kubernetes Flow

When running in Kubernetes, ensure the webhook endpoint is either publicly reachable (via Ingress) to receive hits directly from the Stripe backend or use the Stripe CLI forwarded to a pod port (using `kubectl port-forward`).

## Testing the Happy Path

1. **Initiate Payment**:
   Generate an idempotency key (e.g. `uuidgen`).
   ```bash
   curl -X POST http://localhost:8080/api/v1/payments \
     -H "Content-Type: application/json" \
     -H "Idempotency-Key: your-uuid-here" \
     -d '{
       "amount": 1000,
       "currency": "USD",
       "provider": "stripe",
       "reference_id": "order_123"
     }'
   ```
2. **Simulate Success**:
   ```bash
   stripe trigger payment_intent.succeeded
   ```
3. **Verify Status**:
   Check logs or poll the gateway API to ensure the status transitioned from Pending to Completed.

## Testing Failure Modes

- **Simulate Payment Failure**:
  ```bash
  stripe trigger payment_intent.payment_failed
  ```
  The gateway should transition the payment status to `FAILED`.

## Troubleshooting

### Invalid Webhook Signature
**Symptom**: Webhook requests are rejected with a 401 or 403 status.
**Fix**: Ensure `STRIPE_WEBHOOK_SECRET` exactly matches the secret from `stripe listen` (or the Dashboard endpoint secret) and that development environments are reloaded after setting it.

### Missing Metadata or Referencing Issues
**Symptom**: The gateway receives the webhook but cannot link it to an existing payment.
**Fix**: Ensure the `POST /api/v1/payments` implementation correctly passes internal routing IDs or Gateway Payment IDs into the Stripe `PaymentIntent` metadata.

### Idempotency Mismatches
**Symptom**: Replayed webhooks cause errors or duplicated state changes.
**Fix**: The webhook processing must be fully idempotent. Check the `Idempotency-Key` headers on webhook logs to ensure repeat events from Stripe bypass processing gracefully.

### Unknown Event Type
**Symptom**: The webhook is acknowledged (200 OK) but the payment state isn't updated.
**Fix**: Check application logs. Ensure your gateway explicitly listens for `payment_intent.succeeded` and `payment_intent.payment_failed`, ignoring unsupported notification types.
