# Stripe Demo — Failure Mode Coverage

**Date:** 2026-06-11  
**Endpoint:** `POST /api/v1/webhooks/payment`, `POST /api/v1/payments`  
**Provider:** Stripe (test mode, in-memory repository)

All responses below are from live test runs.

---

## FC-1 — Invalid Webhook Signature

**Scenario:** Webhook arrives with a tampered or incorrect `Stripe-Signature` header.

**Request:**
```bash
curl -X POST http://localhost:8080/api/v1/webhooks/payment \
  -H "Content-Type: application/json" \
  -H "Stripe-Signature: t=1234567890,v1=invalidsignature" \
  -d '{"type":"payment_intent.succeeded",...}'
```

**Response `400 Bad Request`:**
```json
{"error":"Bad Request","message":"Invalid webhook payload or signature","code":400}
```

**Log:**
```
WARN  stripe webhook signature verification failed  error="signature mismatch"
WARN  invalid webhook payload or signature
```

**Verdict:** ✅ Correctly rejected. Payment state unchanged.

---

## FC-2 — Missing Signature Header

**Scenario:** Webhook arrives with no `Stripe-Signature` or `X-Webhook-Signature` header.

**Request:**
```bash
curl -X POST http://localhost:8080/api/v1/webhooks/payment \
  -H "Content-Type: application/json" \
  -d '{"type":"payment_intent.succeeded",...}'
```

**Response `401 Unauthorized`:**
```json
{"error":"Unauthorized","message":"Webhook signature header is required","code":401}
```

**Verdict:** ✅ Correctly rejected before any processing begins.

---

## FC-3 — First Delivery (Baseline)

**Scenario:** Valid signed webhook for a known payment — establishes baseline for replay test.

**Response `200 OK`:**
```json
{"received": true}
```

**Log:**
```
INFO  processing webhook event  event_type=payment.completed  payment_id=pay_xxx
INFO  webhook processed successfully  duration_ms=x
```

**Payment status after:** `pending → processing → completed`  
**Verdict:** ✅ State machine transitions correctly.

---

## FC-4 — Replayed Webhook (Idempotency)

**Scenario:** Identical event delivered a second time after payment is already `completed`.

**Request:** Same payload and a freshly signed `Stripe-Signature` (Stripe re-signs on retry).

**Response `200 OK`:**
```json
{"received": true}
```

**Behaviour:** `ProcessEvent` detects `p.Status == nextStatus` and returns `nil` immediately — no redundant DB write, no state machine error.

**Verdict:** ✅ Idempotent. Replay is safely acknowledged and ignored.

---

## FC-5 — Missing Metadata (No payment_id, Unknown transaction_id)

**Scenario:** Webhook payload carries no `metadata.payment_id` and a `transaction_id` that was never created by this gateway (e.g. a Stripe Dashboard manual trigger or a payment created outside the app).

**Request:**
```bash
curl -X POST http://localhost:8080/api/v1/webhooks/payment \
  -H "Stripe-Signature: t=...,v1=..." \
  -d '{"type":"payment_intent.succeeded","data":{"object":{"id":"pi_unknown","metadata":{}}}}'
```

**Response `404 Not Found`:**
```json
{"error":"Not Found","message":"payment not found","code":404}
```

**Log:**
```
ERROR payment not found by id or provider ref  payment_id=""  provider_ref="pi_unknown_xxx"
WARN  payment not found  error="payment not found by id \"\" or provider ref \"pi_unknown_xxx\": payment not found"
```

**Verdict:** ✅ Returns `404` so Stripe stops retrying. Correct behaviour for payments not originated by this gateway.  
**Workaround for demos:** Always use `--override "payment_intent:metadata[payment_id]=<pay_id>"` when triggering test events.

---

## FC-6 — Duplicate Payment (Idempotency Key Reuse)

**Scenario:** Client retries `POST /api/v1/payments` with the same `X-Idempotency-Key` but different amount and customer.

**Request:**
```bash
curl -X POST http://localhost:8080/api/v1/payments \
  -H "X-Idempotency-Key: demo-1749566038" \   # same key as original
  -d '{"amount":199.99,"currency":"EUR","description":"duplicate attempt","customer_id":"cust_999"}'
```

**Response `201 Created`:**
```json
{
  "payment_id": "pay_20260611044644",
  "status": "completed",
  "amount": 99.99,
  "currency": "USD",
  "transaction_id": "pi_xxx"
}
```

**Behaviour:** Original payment returned unchanged. Body differences (`amount`, `currency`, `customer_id`) are ignored. No new Stripe PaymentIntent created.

**Verdict:** ✅ Idempotent. Correct per standard idempotency semantics.

---

## FC-7 — Failed Payment Webhook

**Scenario:** Stripe fires `payment_intent.payment_failed` for a `pending` payment.

**Request:**
```bash
stripe trigger payment_intent.payment_failed \
  --override "payment_intent:metadata[payment_id]=<pay_id>"
```

**Response `200 OK`:**
```json
{"received": true}
```

**Payment status after:** `pending → processing → failed`  
**Verdict:** ✅ State machine correctly transitions through `processing` before reaching `failed`.

---

## FC-8 — Unhandled Event Type

**Scenario:** Stripe fires an event the gateway doesn't care about (`payment_intent.created`, `charge.updated`, etc.).

**Response `200 OK`:**
```json
{"received": true}
```

**Log:**
```
INFO  ignoring unrecognized provider event type  provider_event_type=payment_intent.created
INFO  ignoring unhandled webhook event type
```

**Verdict:** ✅ Silently acknowledged. Stripe won't retry. No state change.

---

## Test Plan Summary

| # | Case | Method | Expected HTTP | Status |
|---|------|--------|--------------|--------|
| FC-1 | Invalid signature | Bad `v1=` value in header | `400` | ✅ |
| FC-2 | Missing signature header | No `Stripe-Signature` | `401` | ✅ |
| FC-3 | First valid delivery | Signed webhook | `200` | ✅ |
| FC-4 | Replayed webhook | Same event re-delivered | `200` (no-op) | ✅ |
| FC-5 | Missing metadata / unknown PI | Synthetic Stripe event, no `payment_id` | `404` | ✅ |
| FC-6 | Duplicate payment creation | Same idempotency key, different body | `201` original | ✅ |
| FC-7 | Failed payment webhook | `payment_intent.payment_failed` | `200`, status→`failed` | ✅ |
| FC-8 | Unhandled event type | `payment_intent.created` | `200` (ignored) | ✅ |

All 8 failure cases pass. No unhandled `500` errors on invalid webhook input.
