# Stripe Demo Runbook

End-to-end happy path: create a payment → receive webhook → verify `completed`.  
**Total time: ~3 minutes.**

---

## Prerequisites

- Docker running
- Stripe CLI authenticated (`stripe config --list` shows `test_mode_api_key`)
- `.env` has `STRIPE_API_KEY=sk_test_...`

---

## Step 1 — Start the stack

```bash
unset STRIPE_API_KEY STRIPE_WEBHOOK_SECRET
docker compose up --build -d
```

**Expected:**
```
✔ Container payment-gateway-db     Healthy
✔ Container payment-gateway        Started
```

---

## Step 2 — Start webhook forwarder (new terminal)

```bash
stripe listen --forward-to localhost:8080/api/v1/webhooks/payment
```

**Expected:**
```
> Ready! Your webhook signing secret is whsec_test_xxxxx (^C to quit)
```

Copy the `whsec_test_...` value, update `.env`:
```
STRIPE_WEBHOOK_SECRET=whsec_test_xxxxx
```

Restart the app to pick it up:
```bash
docker compose up -d --no-deps payment-gateway
```

---

## Step 3 — Create a payment

```bash
IDEM="demo-$(date +%s)"
PAYMENT=$(curl -s -X POST http://localhost:8080/api/v1/payments \
  -H "Content-Type: application/json" \
  -H "X-Idempotency-Key: $IDEM" \
  -d '{"amount":99.99,"currency":"USD","description":"Stripe demo","customer_id":"cust_123"}')
echo $PAYMENT
PAY_ID=$(echo $PAYMENT | python3 -c "import sys,json; print(json.load(sys.stdin)['payment_id'])")
echo "Payment ID: $PAY_ID"
```

**Expected:**
```json
{
  "payment_id": "pay_20260610xxxxxx",
  "status": "pending",
  "amount": 99.99,
  "currency": "USD",
  "transaction_id": "pi_3Tgo..."
}
```

---

## Step 4 — Trigger Stripe webhook

```bash
stripe trigger payment_intent.succeeded \
  --override "payment_intent:metadata[payment_id]=$PAY_ID"
```

**Expected in stripe listen terminal:**
```
--> payment_intent.succeeded [evt_xxx]
<-- [200] POST http://localhost:8080/api/v1/webhooks/payment
```

---

## Step 5 — Verify final status

```bash
docker compose exec db psql -U user -d payment_gateway \
  -c "SELECT id, status, updated_at FROM payments WHERE id = '$PAY_ID';"
```

**Expected:**
```
        id         |  status   |          updated_at
-------------------+-----------+-------------------------------
 pay_20260610xxxxx | completed | 2026-06-10 17:xx:xx.xxxxxx+00
```

Check logs for the full event trace:
```bash
docker compose logs payment-gateway | grep -v span_ | grep -E "webhook|event|completed"
```

**Expected log sequence:**
```
processing webhook event  event_type=payment.completed  payment_id=pay_20260610xxxxx
webhook processed successfully  event_type=payment.completed  duration_ms=x
```

---

## Quick Reference — What Each Step Proves

| Step | Proves |
|------|--------|
| 1 | App starts, migrations run, Postgres is ready |
| 2 | Webhook signature verification is configured |
| 3 | Stripe PaymentIntent created, gateway returns `pending` |
| 4 | Signature verified, event mapped to domain, state machine triggered |
| 5 | `pending → processing → completed` persisted to Postgres |

---

## Troubleshooting

| Symptom | Fix |
|---------|-----|
| `[400]` on webhook | `STRIPE_WEBHOOK_SECRET` doesn't match `stripe listen` secret — restart app after updating `.env` |
| `[500] payment not found` | `$PAY_ID` wasn't passed in `--override`, Stripe's synthetic PI ID doesn't exist in DB |
| Status stays `pending` | `stripe listen` not running when trigger fired — re-run step 4 |
| `pk_live_ key` error | Shell has stale `STRIPE_API_KEY` env var — run `unset STRIPE_API_KEY` |
